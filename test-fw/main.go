package main

import (
	"device/py32"
	"errors"
	"machine"
	"runtime"
	"time"
)

const (
	pinUartTx   = machine.PB6
	pinUartRx   = machine.PB7
	pinLedRed   = machine.PB0
	pinLedGreen = machine.PB1
	pinI2cSDA   = machine.PA7
	pinI2cSCL   = machine.PA9
)

func main() {
	machine.ConfigureUARTPin(pinUartTx, 0) // TX
	machine.ConfigureUARTPin(pinUartRx, 0) // RX

	pinLedGreen.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinLedRed.Configure(machine.PinConfig{Mode: machine.PinOutput})

	pinI2cSDA.Configure(machine.PinConfig{Mode: machine.PinAlternate})
	pinI2cSDA.SetAltFunc(12)
	pinI2cSCL.Configure(machine.PinConfig{Mode: machine.PinAlternate})
	pinI2cSCL.SetAltFunc(6)

	py32.RCC.APBENR1.SetBits(py32.RCC_APBENR1_I2CEN)

	i2c := NewPyI2CMaster(py32.I2C, 24_000_000, 100_000)

	// data := make([]uint8, 2)

	wr := []uint8{0x02}

	for {

		err := i2c.Write(0x71, &wr)
		if err != nil {
			println("Write error: ", err.Error())
		}

		// read, err := i2c.Read(0x71, &data)
		// if err != nil {
		// 	println("Read error: ", err.Error())
		// } else {
		// 	println("Read: ", read, data[0], data[1])
		// }

		i2c.Stop()

		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		println("Alloc: ", stats.Alloc, " TotalAlloc: ", stats.TotalAlloc, " Sys: ", stats.Sys)

		time.Sleep(500 * time.Millisecond)
	}

	// println("This is the end...")
	// for {
	// }
}

type PyI2CMaster struct {
	peri *py32.I2C_Type
}

var (
	ErrPEC     = errors.New("PEC error")
	ErrOverrun = errors.New("overrun/underrun")
	ErrNoAck   = errors.New("no ACK received")
	ErrArlo    = errors.New("arbitration lost")
	ErrBERR    = errors.New("bus error")
)

func NewPyI2CMaster(peri *py32.I2C_Type, apbClockHz int, i2cClockHz int) *PyI2CMaster {

	peri.SetCR2_FREQ(uint32(apbClockHz / 1e6))
	peri.SetCCR(uint32(apbClockHz / (i2cClockHz * 2)))

	apbClockkTns := 1e9 / apbClockHz
	tRiseMaxNs := 1000 // I2C protocol specification
	trise := uint32((tRiseMaxNs / apbClockkTns) + 1)
	peri.SetTRISE(trise)

	peri.SetCR1_PE(1)

	return &PyI2CMaster{peri: peri}
}

func (i2c *PyI2CMaster) writeByte(b uint8) error {
	i2c.peri.DR.Set(uint32(b))

	for {
		sr1 := i2c.peri.SR1.Get()
		_ = i2c.peri.SR2.Get()

		if sr1&py32.I2C_SR1_PECERR != 0 {
			return ErrPEC
		}

		if sr1&py32.I2C_SR1_OVR != 0 {
			return ErrOverrun
		}

		if sr1&py32.I2C_SR1_AF != 0 {
			return ErrNoAck
		}

		if sr1&py32.I2C_SR1_ARLO != 0 {
			return ErrArlo
		}

		if sr1&py32.I2C_SR1_BERR != 0 {
			return ErrBERR
		}

		if sr1&py32.I2C_SR1_TXE != 0 {
			break
		}

		if sr1&py32.I2C_SR1_RXNE != 0 {
			break
		}

		println(hex32(sr1))

		runtime.Gosched()
	}

	return nil
}

func (i2c *PyI2CMaster) Stop() {
	i2c.peri.CR1.SetBits(py32.I2C_CR1_STOP)
	// for i2c.peri.SR1.HasBits(py32.I2C_SR1_TXE) {
	// 	runtime.Gosched()
	// }
	time.Sleep(10 * time.Millisecond)
}

func (i2c *PyI2CMaster) Write(address uint8, data *[]uint8) error {

	i2c.peri.CR1.SetBits(py32.I2C_CR1_START)
	for !i2c.peri.SR1.HasBits(py32.I2C_SR1_SB) {
		runtime.Gosched()
	}

	err := i2c.writeByte(address << 1)
	if err != nil {
		return err
	}

	for _, b := range *data {

		err := i2c.writeByte(b)
		if err != nil {
			return err
		}

	}

	return nil
}

func (i2c *PyI2CMaster) Read(address uint8, data *[]uint8) (int, error) {

	i2c.peri.CR1.SetBits(py32.I2C_CR1_START)
	for !i2c.peri.SR1.HasBits(py32.I2C_SR1_SB) {
		runtime.Gosched()
	}

	println("read addr")

	err := i2c.writeByte(address<<1 | 1)
	if err != nil {
		return 0, err
	}

	println("reading....")

	read := 0
	for i := range *data {
		sr1 := i2c.peri.SR1.Get()
		_ = i2c.peri.SR2.Get()

		println(hex32(sr1))

		if sr1&py32.I2C_SR1_PECERR != 0 {
			return read, ErrPEC
		}

		if sr1&py32.I2C_SR1_OVR != 0 {
			return read, ErrOverrun
		}

		if sr1&py32.I2C_SR1_AF != 0 {
			return read, ErrNoAck
		}

		if sr1&py32.I2C_SR1_ARLO != 0 {
			return read, ErrArlo
		}

		if sr1&py32.I2C_SR1_BERR != 0 {
			return read, ErrBERR
		}

		if sr1&py32.I2C_SR1_RXNE != 0 {
			println("we have a byte!")
			(*data)[i] = uint8(i2c.peri.DR.Get())
			read++
		}

		runtime.Gosched()
	}

	println("end of read")

	return read, nil
}

func hex32(v uint32) string {
	digits := "0123456789ABCDEF"
	return "0x" + string([]byte{
		digits[v>>28&0xf],
		digits[v>>24&0xf],
		digits[v>>20&0xf],
		digits[v>>16&0xf],
		digits[v>>12&0xf],
		digits[v>>8&0xf],
		digits[v>>4&0xf],
		digits[v&0xf],
	})
}
