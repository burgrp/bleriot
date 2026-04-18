package pan211x

import (
	"errors"
	"runtime"
	"time"
)

// calChannel is used during RF calibration (2485 MHz per SDK comment).
// Must differ from the operating channel. Do not change.
const calChannel = 0x55

// Register addresses (7-bit, Page0 unless noted).
// Shared registers (regPAGE_CFG, regSTATE_CFG) are accessible from both pages.
const (
	regPAGE_CFG      = 0x00 // page select: 0x00=Page0, 0x01=Page1
	regTRX_FIFO      = 0x01 // TX/RX FIFO access point
	regSTATE_CFG     = 0x02 // state machine control (shared across pages)
	regSYS_CFG       = 0x03 // system config / soft reset
	regSPI_CFG       = 0x04 // SPI config: bit7=REG_SPI3_REN enables 3-wire reads
	regXTAL_CFG      = 0x05 // crystal load cap trim (Page0)
	regWMODE_CFG0    = 0x07 // work mode 0: CRC, protocol, whitening, endian
	regWMODE_CFG1    = 0x08 // work mode 1: FIFO size, address width
	regRXPLLEN_CFG   = 0x09 // RX payload length (fixed-length mode)
	regTXPLLEN_CFG   = 0x0A // TX payload length
	regIRQ_MASK      = 0x0B // IRQ mask register
	regPIPE0_RXADDR0 = 0x0F // pipe0 RX address byte 0 (LSB on air)
	regTXADDR0       = 0x14 // TX address byte 0
	regWHITEN_CFG    = 0x1A // whitening seed: bit6:0 = LFSR init value
	regRXPIPE_CFG    = 0x1F // multi-channel pipe enable: bit0=PIPE0_EN (default 1)
	regTXAUTO_CFG    = 0x29 // auto-retransmit config
	regTRXMODE_CFG   = 0x2A // TX/RX mode select
	regRF_DATARATE   = 0x36 // air data rate: bits[5:4] 00=1Mbps 01=2Mbps 11=250kbps
	regRF_CHANNEL    = 0x39 // RF channel: frequency = 2400 + val [MHz]
	regRFIRQFLG      = 0x73 // interrupt flags (write 1 to clear)
)

// STATE_CFG values.
const (
	stateSTB3 = 0x74 // Standby3
	stateTX   = 0x75 // TX mode
	stateRX   = 0x76 // RX mode
)

// RFIRQFLG bits.
const (
	irqTX = 1 << 7 // packet transmission complete
	irqRX = 1 << 0 // correct packet received
)

// Errors returned by Driver methods.
var (
	ErrPayloadTooLarge = errors.New("payload too large")
	ErrTimeout         = errors.New("radio timeout")
	ErrCalibration     = errors.New("calibration failed")
	ErrNoDevice        = errors.New("no device")
)

// DataRate values for RF_DATARATE register (0x36).
// Reserved bits [7:6]=01 and [3:0]=0101 are included in each constant.
const (
	DataRate1Mbps   uint8 = 0x55 // bits[5:4]=00
	DataRate2Mbps   uint8 = 0x65 // bits[5:4]=01
	DataRate250kbps uint8 = 0x75 // bits[5:4]=11
)

// Address is a 4-byte device address stored as a little-endian uint32.
type Address uint32

// Bytes returns the address as 4 bytes in little-endian order.
func (a Address) Bytes() [4]byte {
	return [4]byte{byte(a), byte(a >> 8), byte(a >> 16), byte(a >> 24)}
}

// Config holds the RF parameters for Init.
type Config struct {
	OwnAddr    Address // RX filter: only packets with TXADDR==OwnAddr are accepted
	RFChannel  uint8   // operating frequency: F = 2400 + RFChannel [MHz], range 0-83
	DataRate   uint8   // use DataRate* constants
	PayloadLen uint8   // fixed packet length in bytes (both TX and RX)
}

// Registers abstracts the physical bus (I2C or SPI) for register access.
type Registers interface {
	Read(reg uint8) (uint8, error)
	Write(reg uint8, value uint8) error
	WriteBuffer(reg uint8, data []byte) error
	ReadBuffer(reg uint8, buf []byte) error
}

// Driver controls a PAN211x transceiver.
type Driver struct {
	registers Registers
	cfg       Config
}

// NewDriver creates a Driver using the given Registers implementation and Config.
func NewDriver(registers Registers, cfg Config) *Driver {
	return &Driver{registers: registers, cfg: cfg}
}

// Init wakes the chip, reads factory OTP calibration, configures RF parameters,
// performs RF calibration, and leaves the chip in continuous RX mode.
// Follows the SDK PAN211_Init() sequence exactly.
func (d *Driver) Init() error {
	// ── Step 1: SPI interface init ────────────────────────────────────────────
	if err := d.registers.Write(regPAGE_CFG, 0x00); err != nil {
		return err
	}
	// REG_SPI3_REN=1 enables 3-wire SPI reads. Must be set before entering STB3.
	if err := d.registers.Write(regSPI_CFG, 0x83); err != nil {
		return err
	}

	// ── Step 2: Enter STB3 with soft reset ───────────────────────────────────
	if err := d.registers.Write(regSTATE_CFG, 0x04); err != nil {
		return err
	}
	time.Sleep(1 * time.Millisecond)
	if err := d.registers.Write(regSTATE_CFG, stateSTB3); err != nil {
		return err
	}
	time.Sleep(1 * time.Millisecond)
	if err := d.registers.Write(regSYS_CFG, 0x00); err != nil { // soft reset
		return err
	}
	time.Sleep(1 * time.Millisecond)
	if err := d.registers.Write(regSYS_CFG, 0x02); err != nil { // release soft reset
		return err
	}

	// Verify chip is accessible: SPI_CFG should still read 0x83 after reset.
	v, err := d.registers.Read(regSPI_CFG)
	if err != nil {
		return err
	}
	if v != 0x83 {
		println("SPI_CFG readback:", v)
		return ErrNoDevice
	}

	// ── Step 3: Read factory OTP calibration (Page1) ─────────────────────────
	if err := d.registers.Write(regPAGE_CFG, 0x01); err != nil {
		return err
	}
	if err := d.registers.Write(0x05, 0x00); err != nil {
		return err
	}
	if err := d.registers.Write(0x04, 0x04); err != nil {
		return err
	}
	value2, err := d.registers.Read(0x04)
	if err != nil {
		return err
	}
	if err := d.registers.Write(0x04, 0x08); err != nil {
		return err
	}
	value4, err := d.registers.Read(0x04)
	if err != nil {
		return err
	}
	if err := d.registers.Write(0x05, 0x01); err != nil {
		return err
	}
	println("OTP value2:", value2, "value4:", value4)
	if (value2 & 0x0F) != 1 {
		println("OTP check failed, value2&0x0F:", value2&0x0F)
		return ErrCalibration
	}
	if err := d.registers.Write(0x47, 0x83|((value2>>1)&0x70)); err != nil {
		return err
	}
	calBit := uint8(0)
	if (value2 & 0x10) == 0 {
		calBit = 1
	}
	if err := d.registers.Write(0x43, 0x10|calBit); err != nil {
		return err
	}

	// ── Step 4: Page1 pre-configuration (SDK normal_tx example) ──────────────
	for _, rw := range []struct{ reg, val uint8 }{
		{0x27, 0xAA}, {0x32, 0x1E}, {0x33, 0x19},
		{0x37, 0x15}, {0x3A, 0x14}, {0x3E, 0xF1},
		{0x41, 0xA2}, {0x46, 0xB0}, {0x4C, 0x48},
	} {
		if err := d.registers.Write(rw.reg, rw.val); err != nil {
			return err
		}
	}

	// ── Step 5: Page0 RF configuration ───────────────────────────────────────
	if err := d.registers.Write(regPAGE_CFG, 0x00); err != nil {
		return err
	}
	// Crystal load capacitor from OTP.
	if err := d.registers.Write(regXTAL_CFG, (value4>>4)|0xC0); err != nil {
		return err
	}
	if err := d.registers.Write(regSYS_CFG, 0x06); err != nil {
		return err
	}
	// WMODE_CFG0=0x81: 2-byte CRC, normal mode, whitening disabled, endian=1.
	// [7:6]=10(2-byte CRC) [5:4]=00(normal) [3]=0(no whiten) [2]=0 [1]=0 [0]=1(endian)
	if err := d.registers.Write(regWMODE_CFG0, 0x81); err != nil {
		return err
	}
	// WMODE_CFG1=0xA2: RX_GOON=1, FIFO_128=1, EnDPL=0, ENHANCE=0, 4-byte addr.
	// [7]=1(RX_GOON) [6]=0 [5]=1(FIFO_128) [4]=0(no DPL) [3]=0(no enhance) [1:0]=10(4-byte)
	if err := d.registers.Write(regWMODE_CFG1, 0xA2); err != nil {
		return err
	}
	if err := d.registers.Write(regRXPLLEN_CFG, d.cfg.PayloadLen); err != nil {
		return err
	}
	if err := d.registers.Write(regTXPLLEN_CFG, d.cfg.PayloadLen); err != nil {
		return err
	}
	// 0x7E: TX_IRQ_MSK=0 (enabled), RX_IRQ_MSK=0 (enabled), error IRQs masked.
	if err := d.registers.Write(regIRQ_MASK, 0x7E); err != nil {
		return err
	}
	if err := d.registers.Write(regTXAUTO_CFG, 0x00); err != nil {
		return err
	}
	// TRXMODE_CFG=0x41: TxMode=TX_SINGLE, RxMode=continuous, bit0=1.
	if err := d.registers.Write(regTRXMODE_CFG, 0x41); err != nil {
		return err
	}
	// Whitening seed = 0x7F (SDK default). Both TX and RX must use same seed.
	if err := d.registers.Write(regWHITEN_CFG, 0x7F); err != nil {
		return err
	}
	// Enable PIPE0 explicitly (default=1, but be explicit after soft reset).
	if err := d.registers.Write(regRXPIPE_CFG, 0x01); err != nil {
		return err
	}

	// Own address → PIPE0_RXADDR hardware RX filter (4 bytes, little-endian).
	a := d.cfg.OwnAddr.Bytes()
	for i, reg := range [4]uint8{
		regPIPE0_RXADDR0, regPIPE0_RXADDR0 + 1,
		regPIPE0_RXADDR0 + 2, regPIPE0_RXADDR0 + 3,
	} {
		if err := d.registers.Write(reg, a[i]); err != nil {
			return err
		}
	}

	// Calibration channel, data rate, RF tuning (per SDK ES_Tool V1.2.6, 16 MHz XTAL).
	if err := d.registers.Write(regRF_CHANNEL, calChannel); err != nil {
		return err
	}
	if err := d.registers.Write(regRF_DATARATE, d.cfg.DataRate); err != nil {
		return err
	}
	for _, rw := range []struct{ reg, val uint8 }{
		{0x43, 0x3A}, {0x44, 0x8C},
		{0x55, 0xDD}, {0x56, 0xC9}, {0x57, 0xB7},
		{0x5A, 0x10}, {0x5B, 0xFD}, {0x5C, 0xE9},
		{0x5D, 0xDC}, {0x5E, 0x02}, {0x5F, 0x06},
		{0x60, 0x0E}, {0x61, 0x2E},
		{0x66, 0x34}, {0x68, 0x0D},
		{0x6E, 0x20},
	} {
		if err := d.registers.Write(rw.reg, rw.val); err != nil {
			return err
		}
	}

	// ── Step 6: RF calibration (Page1) ───────────────────────────────────────
	if err := d.registers.Write(regPAGE_CFG, 0x01); err != nil {
		return err
	}
	// VCO calibration.
	if err := d.registers.Write(0x1B, 0x08); err != nil {
		return err
	}
	if err := d.waitBit(0x70, 0x40, 10000); err != nil {
		println("VCO cal timeout")
		return ErrCalibration
	}
	// Thermal calibration: mandatory 55 ms delay.
	if err := d.registers.Write(0x1B, 0x10); err != nil {
		return err
	}
	time.Sleep(55 * time.Millisecond)
	// Frequency calibration: requires RX mode.
	if err := d.registers.Write(regSTATE_CFG, stateRX); err != nil {
		return err
	}
	time.Sleep(200 * time.Microsecond)
	if err := d.registers.Write(0x1B, 0x20); err != nil {
		return err
	}
	if err := d.waitBit(0x7F, 0x80, 10000); err != nil {
		println("freq cal timeout")
		return ErrCalibration
	}
	// Phase calibration 1.
	if err := d.registers.Write(0x1B, 0x40); err != nil {
		return err
	}
	if err := d.waitBit(0x6D, 0x80, 10000); err != nil {
		println("phase1 cal timeout")
		return ErrCalibration
	}
	// Phase calibration 2.
	if err := d.registers.Write(0x1B, 0x80); err != nil {
		return err
	}
	if err := d.waitBit(0x7F, 0x80, 10000); err != nil {
		println("phase2 cal timeout")
		return ErrCalibration
	}
	if err := d.registers.Write(0x1B, 0x00); err != nil {
		return err
	}
	// Back to STB3 and Page0.
	if err := d.registers.Write(regSTATE_CFG, stateSTB3); err != nil {
		return err
	}
	if err := d.registers.Write(regPAGE_CFG, 0x00); err != nil {
		return err
	}

	// Set operating channel, clear all IRQ, enter RX.
	if err := d.registers.Write(regRF_CHANNEL, d.cfg.RFChannel); err != nil {
		return err
	}
	if err := d.registers.Write(regRFIRQFLG, 0xFF); err != nil {
		return err
	}
	return d.registers.Write(regSTATE_CFG, stateRX)
}

// waitBit polls register reg until (val & mask) != 0 or maxIter exhausted.
func (d *Driver) waitBit(reg, mask uint8, maxIter int) error {
	for i := 0; i < maxIter; i++ {
		v, err := d.registers.Read(reg)
		if err != nil {
			return err
		}
		if v&mask != 0 {
			return nil
		}
		runtime.Gosched()
	}
	return ErrCalibration
}

// Send transmits payload to dst. Sets TXADDR to dst before TX so the receiving
// device's PIPE0_RXADDR filter accepts the packet. Blocks for the transmission.
// Re-enters RX mode before returning.
func (d *Driver) Send(dst [4]byte, payload []byte) error {
	if len(payload) > 128 {
		return ErrPayloadTooLarge
	}

	// Enter STB3 before accessing FIFO and address registers (SDK TxStart).
	if err := d.registers.Write(regSTATE_CFG, stateSTB3); err != nil {
		return err
	}

	// Set TXADDR to destination (4 bytes, little-endian).
	for i := uint8(0); i < 4; i++ {
		if err := d.registers.Write(regTXADDR0+i, dst[i]); err != nil {
			return err
		}
	}

	// Update TX payload length in case it differs from the init value.
	if err := d.registers.Write(regTXPLLEN_CFG, uint8(len(payload))); err != nil {
		return err
	}

	// Write payload into TX FIFO.
	if err := d.registers.WriteBuffer(regTRX_FIFO, payload); err != nil {
		return err
	}

	// Clear any stale IRQ flags, then enter TX.
	if err := d.registers.Write(regRFIRQFLG, 0xFF); err != nil {
		return err
	}
	if err := d.registers.Write(regSTATE_CFG, stateTX); err != nil {
		return err
	}

	// Poll for TX-complete IRQ. With TxMode=TX_SINGLE, chip returns to STB3 after TX.
	for i := 0; i < 5000; i++ {
		flags, err := d.registers.Read(regRFIRQFLG)
		if err != nil {
			_ = d.enterRX()
			return err
		}
		if flags&irqTX != 0 {
			_ = d.registers.Write(regRFIRQFLG, 0xFF)
			return d.enterRX()
		}
		runtime.Gosched()
	}

	state, _ := d.registers.Read(regSTATE_CFG)
	irqFlags, _ := d.registers.Read(regRFIRQFLG)
	println("TX timeout STATE_CFG:", state, "RFIRQFLG:", irqFlags)
	_ = d.enterRX()
	return ErrTimeout
}

// Receive checks whether a packet has been received. Non-blocking.
// Fixed-length mode (EnDPL=0): length == PayloadLen == len(buf).
// RX_GOON=1 means the chip stays in RX automatically after each received packet.
func (d *Driver) Receive(buf []byte) (n int, ok bool) {
	flags, err := d.registers.Read(regRFIRQFLG)
	if err != nil || flags&irqRX == 0 {
		return 0, false
	}
	println("RX IRQ fired, flags:", flags)
	if err := d.registers.ReadBuffer(regTRX_FIFO, buf); err != nil {
		_ = d.registers.Write(regRFIRQFLG, 0xFF)
		return 0, false
	}
	_ = d.registers.Write(regRFIRQFLG, irqRX)
	return len(buf), true
}

// DumpState prints key register values to RTT for debugging.
// Always operates on page0 (restores page0 after reading).
func (d *Driver) DumpState() {
	d.registers.Write(regPAGE_CFG, 0x00)
	state, _ := d.registers.Read(regSTATE_CFG)
	irq, _ := d.registers.Read(regRFIRQFLG)
	wmode0, _ := d.registers.Read(regWMODE_CFG0)
	wmode1, _ := d.registers.Read(regWMODE_CFG1)
	ch, _ := d.registers.Read(regRF_CHANNEL)
	dr, _ := d.registers.Read(regRF_DATARATE)
	rxlen, _ := d.registers.Read(regRXPLLEN_CFG)
	a0, _ := d.registers.Read(regPIPE0_RXADDR0)
	a1, _ := d.registers.Read(regPIPE0_RXADDR0 + 1)
	a2, _ := d.registers.Read(regPIPE0_RXADDR0 + 2)
	a3, _ := d.registers.Read(regPIPE0_RXADDR0 + 3)
	println("--- STATE_CFG:", state)
	println("  OPERATE_MODE:", state&0x07, " (4=STB3 5=TX 6=RX)")
	println("--- RFIRQFLG:", irq)
	println("  TX_IRQ:", (irq>>7)&1, " TX_MAX_RT:", (irq>>6)&1)
	println("  RX_ADDR_ERR:", (irq>>5)&1, " RX_CRC_ERR:", (irq>>4)&1)
	println("  RX_LEN_ERR:", (irq>>3)&1, " RX_PID_ERR:", (irq>>2)&1)
	println("  RX_TIMEOUT:", (irq>>1)&1, " RX_IRQ:", irq&1)
	println("--- WMODE_CFG0:", wmode0)
	println("  CRC_MODE:", (wmode0>>6)&3, " (0=off 1=1B 2=2B 3=3B)")
	println("  WORK_MODE:", (wmode0>>4)&3, " (0=XN297L 3=BLE)")
	println("  WHITEN_EN:", (wmode0>>3)&1, " CRC_SKIP_ADDR:", (wmode0>>2)&1)
	println("  TX_NOACK:", (wmode0>>1)&1, " ENDIAN:", wmode0&1, " (0=LE/BLE 1=BE/XN297L)")
	println("--- WMODE_CFG1:", wmode1)
	println("  RX_GOON:", (wmode1>>7)&1, " PRI_EXIT_RX:", (wmode1>>6)&1)
	println("  FIFO_128:", (wmode1>>5)&1, " DPY_EN:", (wmode1>>4)&1)
	println("  ENHANCE:", (wmode1>>3)&1, " ADDR_LEN:", wmode1&3, " (0=2B 1=3B 2=4B 3=5B)")
	println("--- RF")
	println("  CH:", ch, " (freq=", 2400+uint16(ch), "MHz)")
	println("  DATARATE[5:4]:", (dr>>4)&3, " (0=1M 1=2M 3=250k)  raw:", dr)
	println("  RXLEN:", rxlen)
	println("  RXADDR:", a0, a1, a2, a3)
}

// enterRX enters RX from STB3. Used after TX completes.
func (d *Driver) enterRX() error {
	if err := d.registers.Write(regSTATE_CFG, stateSTB3); err != nil {
		return err
	}
	if err := d.registers.Write(regRFIRQFLG, 0xFF); err != nil {
		return err
	}
	return d.registers.Write(regSTATE_CFG, stateRX)
}
