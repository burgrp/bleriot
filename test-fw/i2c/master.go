package i2c

import (
	"device/py32"
	"runtime"
)

type Master struct {
	peri *py32.I2C_Type
}

func NewMaster(peri *py32.I2C_Type, apbClockHz int, i2cClockHz int) *Master {

	peri.SetCR2_FREQ(uint32(apbClockHz / 1e6))
	peri.SetCCR(uint32(apbClockHz / (i2cClockHz * 2)))

	apbClockkTns := 1e9 / apbClockHz
	tRiseMaxNs := 1000 // I2C protocol specification
	trise := uint32((tRiseMaxNs / apbClockkTns) + 1)
	peri.SetTRISE(trise)

	peri.SetCR1_PE(1)

	return &Master{peri: peri}
}

// Write performs START → addr+W → data → STOP.
func (m *Master) Write(addr uint8, data []byte) error {
	for m.peri.SR2.HasBits(py32.I2C_SR2_BUSY) {
		runtime.Gosched()
	}
	if err := m.sendAddr(addr << 1); err != nil {
		m.abort()
		return err
	}
	for _, b := range data {
		if err := m.writeByte(b); err != nil {
			m.abort()
			return err
		}
	}
	m.peri.CR1.SetBits(py32.I2C_CR1_STOP)
	clearErrors(m.peri)
	return nil
}

// Read performs START → addr+R → data → NACK → STOP.
func (m *Master) Read(addr uint8, buf []byte) error {
	for m.peri.SR2.HasBits(py32.I2C_SR2_BUSY) {
		runtime.Gosched()
	}
	return m.recvPhase(addr, buf)
}

// Exchange performs START → addr+W → write → Sr → addr+R → read → NACK → STOP.
func (m *Master) Exchange(addr uint8, write, read []byte) error {
	for m.peri.SR2.HasBits(py32.I2C_SR2_BUSY) {
		runtime.Gosched()
	}
	if err := m.sendAddr(addr << 1); err != nil {
		m.abort()
		return err
	}
	// All bytes except the last wait for TXE&&BTF (EV8_2: fully transmitted).
	for _, b := range write[:len(write)-1] {
		if err := m.writeByte(b); err != nil {
			m.abort()
			return err
		}
	}
	// Last byte waits for TXE only (EV8: byte in shift register, still transmitting).
	// recvPhase then sets START; the hardware finishes the byte and generates Sr.
	if len(write) > 0 {
		m.peri.DR.Set(uint32(write[len(write)-1]))
		for {
			sr1 := m.peri.SR1.Get()
			if err := checkErrors(m.peri); err != nil {
				m.abort()
				return err
			}
			if sr1&py32.I2C_SR1_TXE != 0 {
				break
			}
			runtime.Gosched()
		}
	}
	return m.recvPhase(addr, read)
}

// sendAddr generates START (or Repeated START if bus is active), sends addrByte,
// waits for ADDR, clears ADDR via the SR1+SR2 read sequence.
func (m *Master) sendAddr(addrByte uint8) error {
	m.peri.CR1.SetBits(py32.I2C_CR1_START)
	for !m.peri.SR1.HasBits(py32.I2C_SR1_SB) {
		runtime.Gosched()
	}
	m.peri.DR.Set(uint32(addrByte))
	for {
		sr1 := m.peri.SR1.Get()
		if sr1&py32.I2C_SR1_ADDR != 0 {
			_ = m.peri.SR2.Get() // clears ADDR
			return nil
		}
		if err := checkErrors(m.peri); err != nil {
			return err
		}
		runtime.Gosched()
	}
}

// recvPhase generates START (or Repeated START), sends addr+R, receives len(buf) bytes
// with correct ACK/NACK and STOP scheduling per the RM §25.3.5.5 Method 2.
func (m *Master) recvPhase(addr uint8, buf []byte) error {
	n := len(buf)
	m.peri.CR1.SetBits(py32.I2C_CR1_ACK) // ACK=1 for address phase; adjusted per-byte below
	m.peri.CR1.SetBits(py32.I2C_CR1_START)
	for !m.peri.SR1.HasBits(py32.I2C_SR1_SB) {
		runtime.Gosched()
	}
	m.peri.DR.Set(uint32(addr<<1 | 1))
	for {
		sr1 := m.peri.SR1.Get()
		if sr1&py32.I2C_SR1_ADDR != 0 {
			break
		}
		if err := checkErrors(m.peri); err != nil {
			m.abort()
			return err
		}
		runtime.Gosched()
	}
	// EV6_3 (RM §25.3.5.5): for n==1, clear ACK and set STOP *before* clearing ADDR.
	// Clearing ADDR (reading SR2) starts data clocking; STOP must already be pending.
	if n == 1 {
		m.peri.CR1.ClearBits(py32.I2C_CR1_ACK)
		m.peri.CR1.SetBits(py32.I2C_CR1_STOP)
	}
	_ = m.peri.SR2.Get() // clears ADDR, starts data reception
	for i := range buf {
		for {
			sr1 := m.peri.SR1.Get()
			if sr1&py32.I2C_SR1_RXNE != 0 {
				break
			}
			if err := checkErrors(m.peri); err != nil {
				m.abort()
				return err
			}
			runtime.Gosched()
		}
		if i == n-2 {
			// After second-to-last byte: clear ACK and schedule STOP so the
			// last byte is NACKed and the bus is released after it.
			m.peri.CR1.ClearBits(py32.I2C_CR1_ACK)
			m.peri.CR1.SetBits(py32.I2C_CR1_STOP)
		}
		buf[i] = uint8(m.peri.DR.Get())
	}
	clearErrors(m.peri)
	return nil
}

func (m *Master) writeByte(b uint8) error {
	m.peri.DR.Set(uint32(b))
	for {
		sr1 := m.peri.SR1.Get()
		if err := checkErrors(m.peri); err != nil {
			return err
		}
		if sr1&py32.I2C_SR1_TXE != 0 && sr1&py32.I2C_SR1_BTF != 0 {
			break
		}
		runtime.Gosched()
	}
	return nil
}

func (m *Master) abort() {
	m.peri.CR1.SetBits(py32.I2C_CR1_STOP)
	clearErrors(m.peri)
}
