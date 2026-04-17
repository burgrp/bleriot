package pan211x

import (
	"errors"
	"runtime"
	"time"
)

// maxFIFO is the maximum FIFO size in bytes (shared between TX and RX in normal mode).
const maxFIFO = 128

// calChannel is used during RF calibration (2485 MHz). Must differ from the
// operating channel. Not user-configurable; it is a chip-level requirement.
const calChannel = 0x55

// Register addresses (7-bit, Page0 unless noted). The 8-bit access byte sent over
// I2C is formed by shifting left by 1 and ORing with the R/W bit (0=write, 1=read).
const (
	regPAGE_CFG      = 0x00 // page select: 0x00=Page0, 0x01=Page1 (shared across pages)
	regTRX_FIFO      = 0x01 // TX/RX FIFO access point
	regSTATE_CFG     = 0x02 // operating mode / state machine control (shared across pages)
	regSYS_CFG       = 0x03 // system config / soft reset
	regXTAL_CFG      = 0x05 // crystal load capacitor trim (Page0)
	regI2C_CFG       = 0x06 // I2C interface configuration
	regWMODE_CFG0    = 0x07 // work mode 0: CRC, protocol, whitening, endian
	regWMODE_CFG1    = 0x08 // work mode 1: FIFO size, enhance, address width
	regRXPLLEN_CFG   = 0x09 // RX payload length (fixed-length mode)
	regTXPLLEN_CFG   = 0x0A // TX payload length (bytes in FIFO, not counting auto header/len)
	regIRQ_MASK      = 0x0B // IRQ mask register
	regPIPE0_RXADDR0 = 0x0F // pipe0 RX address byte 0 (transmitted first on air)
	regPIPE0_RXADDR1 = 0x10 // pipe0 RX address byte 1
	regPIPE0_RXADDR2 = 0x11 // pipe0 RX address byte 2
	regPIPE0_RXADDR3 = 0x12 // pipe0 RX address byte 3
	regTXADDR0       = 0x14 // TX address byte 0
	regTXADDR1       = 0x15 // TX address byte 1
	regTXADDR2       = 0x16 // TX address byte 2
	regTXADDR3       = 0x17 // TX address byte 3
	regPKT_EXT_CFG   = 0x19 // HDR_LEN_EXIST[6]: chip auto-inserts BLE header+length
	regWHITEN_CFG    = 0x1A // whitening: WHITEN_SKIP_ADDR[7] | seed[6:0]
	regTXHDR0_CFG    = 0x1B // BLE TX header byte (auto-inserted when HDR_LEN_EXIST=1)
	regTXAUTO_CFG    = 0x29 // auto-retransmit: ARD[7:4] | ARC[3:0]
	regTRXMODE_CFG   = 0x2A // TX/RX mode: TX_SINGLE, RX_CONTINUOUS
	regBLEMATCH_CFG0 = 0x2D // BLE address match filter
	regRF_DATARATE   = 0x36 // air data rate: bits[5:4] 00=1Mbps 01=2Mbps 11=250kbps
	regRF_CHANNEL    = 0x39 // RF channel: frequency = 2400 + val [MHz]
	regMISC_CFG      = 0x6F // misc: I_NDC_PREAMBLE_SEL[5]=BLE preamble, PID_LOW_SEL[4]
	regRFIRQFLG      = 0x73 // interrupt flags (write 1 to clear)
	regSTATUS3       = 0x77 // received payload length
)

// DataRate values for RF_DATARATE_CFG (register 0x36).
// Reserved bits [7:6]=01 and [3:0]=0101 must be preserved.
const (
	DataRate1Mbps   uint8 = 0x55 // bits[5:4] = 00
	DataRate2Mbps   uint8 = 0x65 // bits[5:4] = 01
	DataRate250kbps uint8 = 0x75 // bits[5:4] = 11
)

// STATE_CFG values.
const (
	stateSleep = 0x01 // Sleep (from Deepsleep; ISO_TO_0=0)
	stateSTB3  = 0x74 // Standby3: EN_LS_3V | POR_RSTL | ISO_TO_0 | mode=00
	stateTX    = 0x75 // TX mode
	stateRX    = 0x76 // RX mode
)

// RFIRQFLG bits (write 1 to clear).
const (
	irqTX = 1 << 7 // packet transmission complete
	irqRX = 1 << 0 // correct packet received
)

// Errors returned by Driver methods.
var (
	ErrPayloadTooLarge = errors.New("payload too large")
	ErrTimeout         = errors.New("radio timeout")
	ErrCalibration     = errors.New("calibration failed")
)

// Address is a 4-byte device address stored as a little-endian uint32.
// The zero value (0x00000000) is reserved.
type Address uint32

// Bytes returns the address as 4 bytes in little-endian order, as transmitted
// on the wire and written to radio hardware registers.
func (a Address) Bytes() [4]byte {
	return [4]byte{byte(a), byte(a >> 8), byte(a >> 16), byte(a >> 24)}
}

// Config holds the RF parameters for Init. All fields are caller-supplied so
// the driver is independent of any specific protocol or network.
type Config struct {
	// OwnAddr is this device's address, written to the PIPE0_RXADDR registers.
	// The chip uses it as the RF sync word for hardware filtering:
	// only packets transmitted with TXADDR == OwnAddr are accepted.
	OwnAddr Address

	// RFChannel sets the operating frequency: F = 2400 + RFChannel [MHz].
	// Valid range: 0–83 (2400–2483 MHz).
	RFChannel uint8

	// WhitenCfg is written to WHITEN_CFG (reg 0x1A).
	// Format: WHITEN_SKIP_ADDR[7]=1 | bit-reversed 7-bit LFSR seed[6:0].
	// For BLE channels: seed = channel_index | 0x40, then bit-reversed.
	WhitenCfg uint8

	// DataRate is written to RF_DATARATE_CFG (reg 0x36).
	// Use the DataRate* constants. Reserved bits are included in the constant values.
	DataRate uint8
}

// Registers abstracts the physical bus (I2C or SPI) for register access.
type Registers interface {
	Read(reg uint8) (uint8, error)
	Write(reg uint8, value uint8) error
	// WriteBuffer writes len(data) bytes to reg in a single bus transaction.
	WriteBuffer(reg uint8, data []byte) error
	// ReadBuffer reads len(buf) bytes from reg into buf.
	ReadBuffer(reg uint8, buf []byte) error
}

// Driver controls a PAN211x transceiver. RF parameters are supplied via Config
// at construction; the driver itself carries no protocol-specific knowledge.
type Driver struct {
	registers Registers
	cfg       Config
}

// NewDriver creates a Driver using the given Registers implementation and Config.
func NewDriver(registers Registers, cfg Config) *Driver {
	return &Driver{registers: registers, cfg: cfg}
}

// Init wakes the chip, reads factory OTP calibration, applies Config RF parameters,
// performs RF calibration, and leaves the chip in RX mode.
// Must be called once before Send or Receive.
func (d *Driver) Init() error {
	// ── Step 1: Power-up sequence ────────────────────────────────────────────
	if err := d.registers.Write(regPAGE_CFG, 0x00); err != nil {
		return err
	}
	// Two-step power-up per SDK: partial STB3, then full STB3.
	if err := d.registers.Write(regSTATE_CFG, 0x04); err != nil {
		return err
	}
	time.Sleep(1 * time.Millisecond)
	if err := d.registers.Write(regSTATE_CFG, stateSTB3); err != nil {
		return err
	}
	time.Sleep(1 * time.Millisecond)

	// Soft reset: assert then release.
	if err := d.registers.Write(regSYS_CFG, 0x00); err != nil {
		return err
	}
	time.Sleep(1 * time.Millisecond)
	if err := d.registers.Write(regSYS_CFG, 0x02); err != nil {
		return err
	}

	// Configure chip for 16 MHz crystal (must be set before Page1 calibration reads).
	if err := d.registers.Write(0x37, 0xE0); err != nil {
		return err
	}

	// ── Step 2: Page1 — factory OTP calibration values ───────────────────────
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

	// Verify OTP validity marker (bits[3:0] must equal 1).
	if (value2 & 0x0F) != 1 {
		return ErrCalibration
	}

	// Apply factory calibration to Page1 registers.
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

	// ── Step 3: Page1 pre-configuration ──────────────────────────────────────
	for _, rw := range []struct{ reg, val uint8 }{
		{0x27, 0xAA}, {0x37, 0x15}, {0x3A, 0x14},
		{0x3E, 0xF1}, {0x3F, 0xD2}, {0x40, 0x20}, {0x41, 0xA6}, {0x46, 0xB0}, {0x4C, 0x48},
	} {
		if err := d.registers.Write(rw.reg, rw.val); err != nil {
			return err
		}
	}

	// ── Step 4: Page0 — RF configuration ─────────────────────────────────────
	if err := d.registers.Write(regPAGE_CFG, 0x00); err != nil {
		return err
	}

	// Crystal load capacitor trim from OTP (upper nibble of value4).
	if err := d.registers.Write(regXTAL_CFG, (value4>>4)|0xC0); err != nil {
		return err
	}

	// I2C_CFG: I2C enabled, no IOMUX (poll RFIRQFLG register instead of IRQ pin).
	if err := d.registers.Write(regI2C_CFG, 0x05); err != nil {
		return err
	}

	// WMODE_CFG0: BLE mode, 3-byte CRC, CRC_SKIP_ADDR=1, whitening enabled, little-endian.
	// 0xFC = 0b11_11_1_1_0_0
	if err := d.registers.Write(regWMODE_CFG0, 0xFC); err != nil {
		return err
	}

	// WMODE_CFG1: RX_GOON=1, FIFO_128_EN=1, DPY_EN=1, ENHANCE=0, 4-byte addr width.
	// RX_GOON=1 keeps the chip in RX mode after each received packet.
	// 0xB2 = 0b1_0_1_1_0_0_10
	if err := d.registers.Write(regWMODE_CFG1, 0xB2); err != nil {
		return err
	}

	if err := d.registers.Write(regRXPLLEN_CFG, 0x0C); err != nil { // 12-byte fixed payload
		return err
	}
	if err := d.registers.Write(regTXPLLEN_CFG, 0x13); err != nil { // overridden per Send call
		return err
	}
	if err := d.registers.Write(regIRQ_MASK, 0x7F); err != nil {
		return err
	}

	// Own address → PIPE0_RXADDR: hardware accepts only packets whose sync word
	// (TXADDR on the sender) matches this device's address.
	a := d.cfg.OwnAddr.Bytes()
	for _, rw := range []struct{ reg, val uint8 }{
		{regPIPE0_RXADDR0, a[0]}, {regPIPE0_RXADDR1, a[1]},
		{regPIPE0_RXADDR2, a[2]}, {regPIPE0_RXADDR3, a[3]},
	} {
		if err := d.registers.Write(rw.reg, rw.val); err != nil {
			return err
		}
	}
	// TXADDR is not set here; Send sets it to the destination address before each TX.

	// PKT_EXT_CFG: HDR_LEN_EXIST=1 — chip auto-inserts a 1-byte header (TXHDR0_CFG)
	// and length before FIFO data on TX; strips them on RX.
	if err := d.registers.Write(regPKT_EXT_CFG, 0x60); err != nil {
		return err
	}

	if err := d.registers.Write(regWHITEN_CFG, d.cfg.WhitenCfg); err != nil {
		return err
	}

	// TXHDR0_CFG: header byte prepended by the chip on every TX.
	// Set to 0x00; upper layers are responsible for packet header content.
	if err := d.registers.Write(regTXHDR0_CFG, 0x00); err != nil {
		return err
	}

	if err := d.registers.Write(regTXAUTO_CFG, 0x00); err != nil { // no auto-retransmit
		return err
	}
	if err := d.registers.Write(regTRXMODE_CFG, 0x41); err != nil { // TX single, RX continuous
		return err
	}
	if err := d.registers.Write(regBLEMATCH_CFG0, 0x04); err != nil { // BLE filter
		return err
	}

	// Whitelist defaults (not used; access address filtering is sufficient).
	for _, rw := range []struct{ reg, val uint8 }{
		{0x30, 0xCC}, {0x31, 0xCC}, {0x32, 0xCC}, {0x33, 0xCC}, {0x34, 0xCC}, {0x35, 0x00},
	} {
		if err := d.registers.Write(rw.reg, rw.val); err != nil {
			return err
		}
	}

	// Set calibration channel (2485 MHz); must differ from the operating channel.
	if err := d.registers.Write(regRF_CHANNEL, calChannel); err != nil {
		return err
	}

	// RF tuning registers (16 MHz crystal, generated by ES_Tool V1.2.6).
	for _, rw := range []struct{ reg, val uint8 }{
		{0x43, 0x3A}, {0x44, 0x8C},
		{0x55, 0xDD}, {0x56, 0xC9}, {0x57, 0xB7},
		{0x5A, 0x10}, {0x5B, 0xFD}, {0x5C, 0xE9},
		{0x5D, 0xDC}, {0x5E, 0x02}, {0x5F, 0x06},
		{0x60, 0x0E}, {0x61, 0x2E},
		{0x66, 0x34}, {0x68, 0x0D},
		{0x6E, 0x20}, {regMISC_CFG, 0x10},
	} {
		if err := d.registers.Write(rw.reg, rw.val); err != nil {
			return err
		}
	}

	if err := d.registers.Write(regRF_DATARATE, d.cfg.DataRate); err != nil {
		return err
	}

	// ── Step 5: RF calibration (Page1) ───────────────────────────────────────
	// Calibration is mandatory; without it the PLL cannot lock and TX never completes.
	if err := d.registers.Write(regPAGE_CFG, 0x01); err != nil {
		return err
	}

	// VCO calibration.
	if err := d.registers.Write(0x1B, 0x08); err != nil {
		return err
	}
	if err := d.waitBit(0x70, 0x40, 10000); err != nil {
		return ErrCalibration
	}

	// Thermal calibration (55 ms settling).
	if err := d.registers.Write(0x1B, 0x10); err != nil {
		return err
	}
	time.Sleep(55 * time.Millisecond)

	// Enter RX mode (required for frequency calibration).
	if err := d.registers.Write(regSTATE_CFG, stateRX); err != nil {
		return err
	}
	time.Sleep(200 * time.Microsecond)

	// Frequency calibration.
	if err := d.registers.Write(0x1B, 0x20); err != nil {
		return err
	}
	if err := d.waitBit(0x7F, 0x80, 10000); err != nil {
		return ErrCalibration
	}

	// Phase calibration 1.
	if err := d.registers.Write(0x1B, 0x40); err != nil {
		return err
	}
	if err := d.waitBit(0x6D, 0x80, 10000); err != nil {
		return ErrCalibration
	}

	// Phase calibration 2.
	if err := d.registers.Write(0x1B, 0x80); err != nil {
		return err
	}
	if err := d.waitBit(0x7F, 0x80, 10000); err != nil {
		return ErrCalibration
	}

	// Stop calibration and return to STB3.
	if err := d.registers.Write(0x1B, 0x00); err != nil {
		return err
	}
	if err := d.registers.Write(regSTATE_CFG, stateSTB3); err != nil {
		return err
	}

	// Back to Page0, set operating channel, enter RX.
	if err := d.registers.Write(regPAGE_CFG, 0x00); err != nil {
		return err
	}
	if err := d.registers.Write(regRF_CHANNEL, d.cfg.RFChannel); err != nil {
		return err
	}

	return d.enterRX()
}

// waitBit polls register reg until (val & mask) != 0 or iterations are exhausted.
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

// enterRX clears all pending IRQ flags and puts the chip into RX mode.
// With RX_GOON=1 (WMODE_CFG1 bit7) the chip stays in RX after each received packet.
func (d *Driver) enterRX() error {
	if err := d.registers.Write(regRFIRQFLG, 0xFF); err != nil {
		return err
	}
	return d.registers.Write(regSTATE_CFG, stateRX)
}

// Send transmits payload to dst. Sets TXADDR to dst before TX so the receiving
// device's hardware (PIPE0_RXADDR == dst) accepts the packet. Blocks only for
// the duration of the air transmission. Re-enters RX mode before returning.
// payload must be at most 128 bytes.
func (d *Driver) Send(dst [4]byte, payload []byte) error {
	if len(payload) > maxFIFO {
		return ErrPayloadTooLarge
	}

	// Enter STB3 (FIFO and address register access requires STB3 or above).
	if err := d.registers.Write(regSTATE_CFG, stateSTB3); err != nil {
		return err
	}

	// Set TXADDR to destination: the chip uses this as the RF sync word,
	// which must match the receiver's PIPE0_RXADDR for hardware filtering.
	for i, rw := range [4]struct{ reg uint8 }{
		{regTXADDR0}, {regTXADDR1}, {regTXADDR2}, {regTXADDR3},
	} {
		if err := d.registers.Write(rw.reg, dst[i]); err != nil {
			return err
		}
	}

	// Set TX payload length (number of bytes in FIFO, excluding auto header+length).
	if err := d.registers.Write(regTXPLLEN_CFG, uint8(len(payload))); err != nil {
		return err
	}

	// Write payload into TX FIFO.
	if err := d.registers.WriteBuffer(regTRX_FIFO, payload); err != nil {
		return err
	}

	// Clear all stale IRQ flags before entering TX.
	if err := d.registers.Write(regRFIRQFLG, 0xFF); err != nil {
		return err
	}

	// Enter TX mode. Chip returns to STB3 automatically after one packet (TX_SINGLE).
	if err := d.registers.Write(regSTATE_CFG, stateTX); err != nil {
		return err
	}

	// Poll RFIRQFLG for the TX-complete interrupt.
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

	// Timeout: attempt recovery.
	state, _ := d.registers.Read(regSTATE_CFG)
	irqFlags, _ := d.registers.Read(regRFIRQFLG)
	println("TX timeout: STATE_CFG=", state, "RFIRQFLG=", irqFlags)
	_ = d.enterRX()
	return ErrTimeout
}

// Receive checks whether a packet has been received. If one is available it is
// copied into buf and (n, true) is returned. If no packet is waiting, returns
// (0, false) immediately without blocking.
// With RX_GOON=1 the chip stays in RX mode automatically after each received packet.
func (d *Driver) Receive(buf []byte) (n int, ok bool) {
	flags, err := d.registers.Read(regRFIRQFLG)
	if err != nil || flags&irqRX == 0 {
		return 0, false
	}

	rxLen, err := d.registers.Read(regSTATUS3)
	if err != nil {
		_ = d.registers.Write(regRFIRQFLG, 0xFF)
		return 0, false
	}

	n = int(rxLen)
	if n > len(buf) {
		n = len(buf)
	}

	if err := d.registers.ReadBuffer(regTRX_FIFO, buf[:n]); err != nil {
		_ = d.registers.Write(regRFIRQFLG, 0xFF)
		return 0, false
	}

	_ = d.registers.Write(regRFIRQFLG, irqRX)
	return n, true
}
