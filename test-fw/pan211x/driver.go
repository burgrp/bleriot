package pan211x

import (
	"errors"
	"runtime"
	"time"
)

// maxFIFO is the maximum FIFO size in bytes (shared between TX and RX in normal mode).
const maxFIFO = 128

// BLE advertising access address (per BLE spec, little-endian byte order).
const (
	bleAccAddr0 = 0xD6
	bleAccAddr1 = 0xBE
	bleAccAddr2 = 0x89
	bleAccAddr3 = 0x8E
)

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
	regPIPE0_RXADDR0 = 0x0F // pipe0 RX address byte 0
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
	regRF_CHANNEL    = 0x39 // RF channel: frequency = 2400 + val [MHz]
	regMISC_CFG      = 0x6F // misc: I_NDC_PREAMBLE_SEL[5]=BLE preamble, PID_LOW_SEL[4]
	regRFIRQFLG      = 0x73 // interrupt flags (write 1 to clear)
	regSTATUS3       = 0x77 // received payload length
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

// Registers abstracts the physical bus (I2C or SPI) for register access.
type Registers interface {
	Read(reg uint8) (uint8, error)
	Write(reg uint8, value uint8) error
	// WriteBuffer writes len(data) bytes to reg in a single bus transaction.
	WriteBuffer(reg uint8, data []byte) error
	// ReadBuffer reads len(buf) bytes from reg into buf.
	ReadBuffer(reg uint8, buf []byte) error
}

// BLEChannel holds the register values for a BLE advertising channel.
type BLEChannel struct {
	rfCh      uint8 // RF_CH register: frequency = 2400 + rfCh [MHz]
	whitenCfg uint8 // WHITEN_CFG: WHITEN_SKIP_ADDR[7]=1 | bit-reversed 7-bit BLE LFSR seed
}

// BLE advertising channel configurations.
// The PAN211x requires the BLE LFSR seed (channel_index | 0x40) to be bit-reversed:
//
//	Ch37 (2402 MHz, index 37): seed 0x65 → bit-rev → 0x53, WHITEN_CFG = 0x80|0x53 = 0xD3
//	Ch38 (2426 MHz, index 38): seed 0x66 → bit-rev → 0x33, WHITEN_CFG = 0x80|0x33 = 0xB3
//	Ch39 (2480 MHz, index 39): seed 0x67 → bit-rev → 0x73, WHITEN_CFG = 0x80|0x73 = 0xF3
var (
	BLECh37 = BLEChannel{rfCh: 2, whitenCfg: 0xD3}  // 2402 MHz
	BLECh38 = BLEChannel{rfCh: 26, whitenCfg: 0xB3} // 2426 MHz
	BLECh39 = BLEChannel{rfCh: 80, whitenCfg: 0xF3} // 2480 MHz
)

// Driver provides BLE packet send and receive over the PAN211x transceiver.
type Driver struct {
	registers Registers
}

// NewDriver creates a Driver using the given Registers implementation.
func NewDriver(registers Registers) *Driver {
	return &Driver{registers: registers}
}

// Init wakes the chip, performs RF calibration, and configures it for BLE advertising.
// Must be called once before Send or Receive.
func (d *Driver) Init(ch BLEChannel) error {
	// ── Step 1: Power-up sequence ────────────────────────────────────────────
	// Ensure Page0 selected.
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

	// ── Step 4: Page0 — BLE configuration ────────────────────────────────────
	if err := d.registers.Write(regPAGE_CFG, 0x00); err != nil {
		return err
	}

	// Crystal load capacitor trim from OTP (upper nibble of value4).
	if err := d.registers.Write(regXTAL_CFG, (value4>>4)|0xC0); err != nil {
		return err
	}

	// I2C_CFG: I2C enabled, no IOMUX (we poll RFIRQFLG register, not a pin).
	if err := d.registers.Write(regI2C_CFG, 0x05); err != nil {
		return err
	}

	// WMODE_CFG0: BLE mode, 3-byte CRC, CRC_SKIP_ADDR=1, whitening enabled, little-endian.
	// 0xFC = 0b11_11_1_1_0_0
	if err := d.registers.Write(regWMODE_CFG0, 0xFC); err != nil {
		return err
	}

	// WMODE_CFG1: RX_GOON=1, FIFO_128_EN=1, DPY_EN=1, ENHANCE=0, 4-byte addr width.
	// 0xB2 = 0b1_0_1_1_0_0_10
	if err := d.registers.Write(regWMODE_CFG1, 0xB2); err != nil {
		return err
	}

	if err := d.registers.Write(regRXPLLEN_CFG, 0x13); err != nil { // RxLen default
		return err
	}
	if err := d.registers.Write(regTXPLLEN_CFG, 0x13); err != nil { // TxLen default (overridden in Send)
		return err
	}
	if err := d.registers.Write(regIRQ_MASK, 0x7F); err != nil {
		return err
	}

	// BLE advertising access address 0x8E89BED6 for pipe0 RX and TX.
	for _, rw := range []struct{ reg, val uint8 }{
		{regPIPE0_RXADDR0, bleAccAddr0}, {regPIPE0_RXADDR1, bleAccAddr1},
		{regPIPE0_RXADDR2, bleAccAddr2}, {regPIPE0_RXADDR3, bleAccAddr3},
		{regTXADDR0, bleAccAddr0}, {regTXADDR1, bleAccAddr1},
		{regTXADDR2, bleAccAddr2}, {regTXADDR3, bleAccAddr3},
	} {
		if err := d.registers.Write(rw.reg, rw.val); err != nil {
			return err
		}
	}

	// PKT_EXT_CFG: HDR_LEN_EXIST=1 — chip auto-inserts BLE header (from TXHDR0_CFG)
	// and length (from TxLen) before FIFO data. FIFO must contain AdvA+AdvData only.
	if err := d.registers.Write(regPKT_EXT_CFG, 0x60); err != nil {
		return err
	}

	// Whitening config: initial channel (updated per packet via SetChannel).
	if err := d.registers.Write(regWHITEN_CFG, ch.whitenCfg); err != nil {
		return err
	}

	// TXHDR0_CFG: BLE PDU header byte = ADV_NONCONN_IND(0x02) | TxAdd=1(0x40) = 0x42.
	if err := d.registers.Write(regTXHDR0_CFG, 0x42); err != nil {
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

	// Whitelist (SDK defaults; not used in TX-only mode).
	for _, rw := range []struct{ reg, val uint8 }{
		{0x30, 0xCC}, {0x31, 0xCC}, {0x32, 0xCC}, {0x33, 0xCC}, {0x34, 0xCC}, {0x35, 0x00},
	} {
		if err := d.registers.Write(rw.reg, rw.val); err != nil {
			return err
		}
	}

	// Set calibration channel (2485 MHz); must be set before RF calibration.
	if err := d.registers.Write(regRF_CHANNEL, 0x55); err != nil {
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
		{0x6E, 0x20}, {regMISC_CFG, 0x10}, // MISC_CFG: PID_LOW_SEL[4]=1 (SDK default for BLE mode)
	} {
		if err := d.registers.Write(rw.reg, rw.val); err != nil {
			return err
		}
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

	// Back to Page0, clear all IRQs, set actual RF channel.
	if err := d.registers.Write(regPAGE_CFG, 0x00); err != nil {
		return err
	}
	if err := d.registers.Write(regRFIRQFLG, 0xFF); err != nil {
		return err
	}
	if err := d.registers.Write(regRF_CHANNEL, ch.rfCh); err != nil {
		return err
	}

	return nil
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

// SetChannel switches the RF channel and whitening seed for the next transmission.
// Call this between Send() calls to cycle through BLE advertising channels.
func (d *Driver) SetChannel(ch BLEChannel) error {
	if err := d.registers.Write(regRF_CHANNEL, ch.rfCh); err != nil {
		return err
	}
	return d.registers.Write(regWHITEN_CFG, ch.whitenCfg)
}

// Send transmits a BLE packet. payload must contain AdvA (6 bytes) followed by
// AdvData — no PDU header or length byte; the chip auto-inserts those from
// TXHDR0_CFG and TxLen when HDR_LEN_EXIST=1 (reg 0x19=0x60).
// Maximum payload is 128 bytes; BLE advertising is typically ≤37 bytes.
func (d *Driver) Send(payload []byte) error {
	if len(payload) > maxFIFO {
		return ErrPayloadTooLarge
	}

	// Enter STB3 (FIFO access requires STB3 or above).
	if err := d.registers.Write(regSTATE_CFG, stateSTB3); err != nil {
		return err
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
			_ = d.registers.Write(regSTATE_CFG, stateSTB3)
			return err
		}
		if flags&irqTX != 0 {
			return d.registers.Write(regRFIRQFLG, 0xFF)
		}
		runtime.Gosched()
	}

	// Timeout: print diagnostics and attempt recovery.
	state, _ := d.registers.Read(regSTATE_CFG)
	irqFlags, _ := d.registers.Read(regRFIRQFLG)
	println("TX timeout: STATE_CFG=", state, "RFIRQFLG=", irqFlags)
	_ = d.registers.Write(regSTATE_CFG, stateSTB3)
	return ErrTimeout
}

// Receive switches the chip to RX mode and waits for one BLE packet.
// The received PDU bytes are written into buf; the actual number of bytes received
// is returned. len(buf) should be at least 39 bytes for a full BLE ADV PDU.
func (d *Driver) Receive(buf []byte) (int, error) {
	if err := d.registers.Write(regRFIRQFLG, irqRX); err != nil {
		return 0, err
	}
	if err := d.registers.Write(regSTATE_CFG, stateRX); err != nil {
		return 0, err
	}

	for i := 0; i < 500000; i++ {
		flags, err := d.registers.Read(regRFIRQFLG)
		if err != nil {
			_ = d.registers.Write(regSTATE_CFG, stateSTB3)
			return 0, err
		}
		if flags&irqRX != 0 {
			rxLen, err := d.registers.Read(regSTATUS3)
			if err != nil {
				_ = d.registers.Write(regSTATE_CFG, stateSTB3)
				return 0, err
			}
			n := int(rxLen)
			if n > len(buf) {
				n = len(buf)
			}
			if err := d.registers.ReadBuffer(regTRX_FIFO, buf[:n]); err != nil {
				_ = d.registers.Write(regSTATE_CFG, stateSTB3)
				return 0, err
			}
			if err := d.registers.Write(regRFIRQFLG, irqRX); err != nil {
				return 0, err
			}
			return n, d.registers.Write(regSTATE_CFG, stateSTB3)
		}
		runtime.Gosched()
	}

	_ = d.registers.Write(regSTATE_CFG, stateSTB3)
	return 0, ErrTimeout
}
