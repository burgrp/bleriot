package pan211x

import (
	"errors"
	"runtime"
	"time"
)

var (
	ErrPayloadTooLarge = errors.New("payload too large")
	ErrTimeout         = errors.New("radio timeout")
	ErrCalibration     = errors.New("calibration failed")
	ErrNoDevice        = errors.New("no device")
)

type Address [5]byte

type BitRate uint8

const (
	BitRate250Kbps BitRate = 0
	BitRate1Mbps   BitRate = 1
	BitRate2Mbps   BitRate = 2
)

// Registers abstracts the physical bus (I2C or SPI) for register access.
type Registers interface {
	Read(reg uint8) (uint8, error)
	Write(reg uint8, value uint8) error
	WriteBuffer(reg uint8, data []byte) error
	ReadBuffer(reg uint8, buf []byte) error
}

type ConfigXN297L struct {
	BitRate    BitRate
	PayloadLen uint8
}

type Driver struct {
	registers  Registers
	rxPipeMask uint8 // RXPIPE_CFG bits [5:0] — which pipes are active
	payloadLen uint8
	inRX       bool
}

func NewDriver(registers Registers) *Driver {
	return &Driver{registers: registers}
}

// pollBit reads reg until (val & bit) != 0, yielding the scheduler between reads.
func (d *Driver) pollBit(reg, bit uint8, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		v, err := d.registers.Read(reg)
		if err != nil {
			return err
		}
		if v&bit != 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return ErrTimeout
		}
		runtime.Gosched()
	}
}

func (d *Driver) enterRX() error {
	if err := d.registers.Write(STATE_CFG, STATE_STB3); err != nil {
		return err
	}
	if err := d.registers.Write(RFIRQFLG, IRQ_ALL); err != nil {
		return err
	}
	if err := d.registers.Write(STATE_CFG, STATE_RX); err != nil {
		return err
	}
	d.inRX = true
	return nil
}

// ensureSTB3 transitions to STB3 if currently in RX mode.
func (d *Driver) ensureSTB3() error {
	if d.inRX {
		if err := d.registers.Write(STATE_CFG, STATE_STB3); err != nil {
			return err
		}
		d.inRX = false
	}
	return nil
}

// writeAddr writes a 5-byte address as individual register writes starting at startReg.
func (d *Driver) writeAddr(startReg uint8, addr Address) error {
	for i, b := range addr {
		if err := d.registers.Write(startReg+uint8(i), b); err != nil {
			return err
		}
	}
	return nil
}

// InitXN297L initialises the chip for XN297L Normal mode (fixed payload, no auto-ACK).
// Crystal: 16 MHz. TX power: 9 dBm. Caller must call SetChannel after this returns.
func (d *Driver) InitXN297L(cfg ConfigXN297L) error {
	d.payloadLen = cfg.PayloadLen
	d.rxPipeMask = PIPE0_EN
	d.inRX = false
	r := d.registers

	// Step 1: ensure Page 0.
	if err := r.Write(PAGE_CFG, 0x00); err != nil {
		return err
	}

	// Step 2: enter STB3 with soft reset to bring all Page 0 registers to defaults.
	if err := r.Write(STATE_CFG, STATE_STB3_INIT); err != nil {
		return err
	}
	time.Sleep(time.Millisecond)
	if err := r.Write(STATE_CFG, STATE_STB3); err != nil {
		return err
	}
	time.Sleep(time.Millisecond)
	if err := r.Write(SYS_CFG, SYS_CFG_RESET); err != nil {
		return err
	}
	time.Sleep(time.Millisecond)
	if err := r.Write(SYS_CFG, SYS_CFG_RELEASE); err != nil {
		return err
	}
	// Required for 16 MHz crystal before any Page 1 access.
	if err := r.Write(RF_OSC_CFG, RF_OSC_CFG_16MHZ); err != nil {
		return err
	}

	// Step 3: read eFuse factory calibration from Page 1.
	if err := r.Write(PAGE_CFG, 0x01); err != nil {
		return err
	}
	if err := r.Write(P1_OTP_CTL, OTP_CTL_START); err != nil {
		return err
	}
	if err := r.Write(P1_OTP_DATA, OTP_READ_WORD2); err != nil {
		return err
	}
	value2, err := r.Read(P1_OTP_DATA)
	if err != nil {
		return err
	}
	if err := r.Write(P1_OTP_DATA, OTP_READ_WORD4); err != nil {
		return err
	}
	value4, err := r.Read(P1_OTP_DATA)
	if err != nil {
		return err
	}
	if err := r.Write(P1_OTP_CTL, OTP_CTL_STOP); err != nil {
		return err
	}
	if value2&OTP_VALID_MASK != OTP_VALID_VAL {
		return ErrNoDevice
	}
	// Apply eFuse-derived trim values while still on Page 1.
	calBit := uint8(0)
	if value2&OTP_CAL_MASK == 0 {
		calBit = 1
	}
	if err := r.Write(P1_PA_TUNE_47, 0x83|((value2>>1)&0x70)); err != nil {
		return err
	}
	if err := r.Write(P1_PA_TUNE_43, 0x10|calBit); err != nil {
		return err
	}

	// Step 4: Page 1 pre-configuration — XN297L Normal, 16 MHz crystal.
	if err := r.Write(P1_RF_TUNE_27, 0xAA); err != nil {
		return err
	}
	if err := r.Write(P1_RF_TUNE_32, 0x1E); err != nil {
		return err
	}
	if err := r.Write(P1_RF_TUNE_33, 0x19); err != nil {
		return err
	}
	if err := r.Write(P1_RF_TUNE_37, 0x15); err != nil {
		return err
	}
	p1Tune3A := uint8(0x14)
	if cfg.BitRate == BitRate250Kbps {
		p1Tune3A |= P1_RF_TUNE_3A_FLTRTUNE_250K
	}
	if err := r.Write(P1_RF_TUNE_3A, p1Tune3A); err != nil {
		return err
	}
	if err := r.Write(P1_RF_TUNE_3E, 0xF1); err != nil {
		return err
	}
	if err := r.Write(P1_RF_TUNE_3F, 0xD2); err != nil { // 16 MHz only
		return err
	}
	if err := r.Write(P1_RF_TUNE_40, 0x20); err != nil { // 16 MHz only
		return err
	}
	if err := r.Write(P1_VCO_PA_CTL, P1_VCO_PA_CTL_16MHZ); err != nil {
		return err
	}
	if err := r.Write(P1_PA_BIAS, PA_BIAS_9DBM); err != nil {
		return err
	}
	switch cfg.BitRate {
	case BitRate2Mbps:
		if err := r.Write(P1_TX_DAC, P1_TX_DAC_GC_BIT|P1_TX_DAC_ISEL_4); err != nil {
			return err
		}
		if err := r.Write(P1_RF_TUNE_4C, 0x48|P1_RF_TUNE_4C_TX_DAC_BW); err != nil {
			return err
		}
	case BitRate250Kbps:
		if err := r.Write(P1_TX_DAC, P1_TX_DAC_ISEL_4); err != nil {
			return err
		}
		if err := r.Write(P1_RF_TUNE_4C, 0x48); err != nil {
			return err
		}
	default: // 1 Mbps
		if err := r.Write(P1_RF_TUNE_4C, 0x48); err != nil {
			return err
		}
	}

	// Step 5: Page 0 configuration.
	if err := r.Write(PAGE_CFG, 0x00); err != nil {
		return err
	}
	if err := r.Write(XTAL_CFG, (value4>>4)|0xC0); err != nil {
		return err
	}
	if err := r.Write(LP_CFG, 0x0D); err != nil { // IRQ routed onto SDA (I2C mode)
		return err
	}
	// WMODE_CFG0 = 0x89: 2-byte CRC | XN297L mode | whitening | MSB-first
	if err := r.Write(WMODE_CFG0, CRC_2B|WORK_MODE_XN297L|WHITEN_EN_BIT|ENDIAN_BIG); err != nil {
		return err
	}
	// WMODE_CFG1 = 0xA3: RX_GOON | 128-byte FIFO | Normal (no ENHANCE) | 5-byte addr
	if err := r.Write(WMODE_CFG1, RX_GOON_BIT|FIFO_128_BIT|ADDR_5B); err != nil {
		return err
	}
	if err := r.Write(RXPLLEN_CFG, cfg.PayloadLen); err != nil {
		return err
	}
	if err := r.Write(TXPLLEN_CFG, cfg.PayloadLen); err != nil {
		return err
	}
	if err := r.Write(RFIRQ_CFG, 0x7F); err != nil { // unmask TX_IRQ only; RX flag still set in RFIRQFLG
		return err
	}
	if err := r.Write(TXAUTO_CFG, 0x00); err != nil { // no auto-retransmit
		return err
	}
	if err := r.Write(TRXMODE_CFG, TRXMODE_CFG_NORMAL); err != nil { // single TX, continuous RX
		return err
	}
	if err := r.Write(WHITEN_CFG, WHITEN_DEFAULT); err != nil {
		return err
	}
	if err := r.Write(RF_CHANNEL_CFG, RF_CH_CAL); err != nil { // calibration freq; replaced after Step 6
		return err
	}
	// RF_DATARATE_CFG not written — chip defaults to 1 Mbps (ES_Tool omits it on 16 MHz).
	// RF_PA_MODE_CFG: 16 MHz values (EN_RXADCCLK=0, unlike 32 MHz).
	switch cfg.BitRate {
	case BitRate2Mbps:
		if err := r.Write(RF_PA_MODE_CFG, 0x36); err != nil { // TXPA_MODE=11, FSYNVCO_TXCTK=1, RXFLTR=2
			return err
		}
	case BitRate250Kbps:
		if err := r.Write(RF_PA_MODE_CFG, 0x03); err != nil { // TXPA_MODE=00, RXFLTR=3
			return err
		}
	default: // 1 Mbps
		if err := r.Write(RF_PA_MODE_CFG, 0x32); err != nil { // TXPA_MODE=11, RXFLTR=2
			return err
		}
	}
	if err := r.Write(RF_PA_POUT_CFG, RF_PA_POUT_CFG_9DBM); err != nil {
		return err
	}
	if err := r.Write(RF_RSSI_TH1, 0xDD); err != nil {
		return err
	}
	if err := r.Write(RF_RSSI_TH2, 0xC9); err != nil {
		return err
	}
	if err := r.Write(RF_RSSI_TH3, 0xB7); err != nil {
		return err
	}
	// RF_RSSI_FIX0–3 (0x5A–0x5D) and RF_GAIN_WORD0–3 (0x5E–0x61) are NOT written
	// on 16 MHz — ES_Tool omits them entirely for this crystal frequency.
	if err := r.Write(RF_TX_ANA_TIME, 0x64); err != nil { // 16 MHz value
		return err
	}
	if err := r.Write(RF_RX_PLL_SETUP, 0x19); err != nil { // 16 MHz value
		return err
	}
	if err := r.Write(RF_PA_RAMP_DLY, 0x40); err != nil { // 16 MHz value
		return err
	}

	// Step 6: RF calibration — 5 phases in strict order on Page 1.
	if err := r.Write(PAGE_CFG, 0x01); err != nil {
		return err
	}

	// Phase 1: VCO calibration.
	if err := r.Write(P1_CAL_CTL, CAL_VCO); err != nil {
		return err
	}
	if err := d.pollBit(P1_CAL_STATUS_VCO, CAL_VCO_DONE_BIT, 5*time.Millisecond); err != nil {
		return ErrCalibration
	}

	// Phase 2: thermal (2-point) calibration — mandatory 55 ms, no status register.
	if err := r.Write(P1_CAL_CTL, CAL_THERMAL); err != nil {
		return err
	}
	time.Sleep(55 * time.Millisecond)

	// Phase 3: frequency offset calibration.
	// STATE_CFG is a shared register, writable from Page 1.
	// The chip must be in RX mode and the RFPLL must lock (≥200 µs) before triggering.
	if err := r.Write(STATE_CFG, STATE_RX); err != nil {
		return err
	}
	time.Sleep(200 * time.Microsecond)
	if err := r.Write(P1_CAL_CTL, CAL_FREQ); err != nil {
		return err
	}
	if err := d.pollBit(P1_CAL_STATUS_DONE, CAL_DONE_BIT, 5*time.Millisecond); err != nil {
		return ErrCalibration
	}

	// Phase 4: BW / filter calibration.
	if err := r.Write(P1_CAL_CTL, CAL_PHASE1); err != nil {
		return err
	}
	if err := d.pollBit(P1_CAL_STATUS_PHASE1, CAL_PHASE1_DONE_BIT, 5*time.Millisecond); err != nil {
		return ErrCalibration
	}

	// Phase 5: DC offset calibration.
	if err := r.Write(P1_CAL_CTL, CAL_PHASE2); err != nil {
		return err
	}
	if err := d.pollBit(P1_CAL_STATUS_DONE, CAL_DONE_BIT, 5*time.Millisecond); err != nil {
		return ErrCalibration
	}

	// Wrap up: stop FSM, return to STB3 on Page 0, clear all IRQ flags.
	if err := r.Write(P1_CAL_CTL, CAL_STOP); err != nil {
		return err
	}
	if err := r.Write(STATE_CFG, STATE_STB3); err != nil {
		return err
	}
	if err := r.Write(PAGE_CFG, 0x00); err != nil {
		return err
	}
	return r.Write(RFIRQFLG, IRQ_ALL)
	// RF_CHANNEL_CFG is still RF_CH_CAL; caller must call SetChannel before use.
}

// SetChannel sets the RF channel. ch = frequency_MHz − 2400 (valid 0–83).
// Can be called at any time; temporarily leaves RX mode if active.
func (d *Driver) SetChannel(channel uint8) error {
	wasRX := d.inRX
	if err := d.ensureSTB3(); err != nil {
		return err
	}
	if err := d.registers.Write(RF_CHANNEL_CFG, channel); err != nil {
		return err
	}
	if wasRX {
		return d.enterRX()
	}
	return nil
}

// EnableRxAddress sets the receive address for pipe pipeIndex (0–5) and enables the pipe.
// Pipes 0 and 1 use the full 5-byte addr. Pipes 2–5 use only addr[0] (LSB);
// their upper 4 bytes are shared with pipe 1 and must be set via pipe 1 first.
func (d *Driver) EnableRxAddress(pipeIndex uint8, addr Address) error {
	if pipeIndex > 5 {
		return errors.New("invalid pipe index")
	}
	wasRX := d.inRX
	if err := d.ensureSTB3(); err != nil {
		return err
	}
	switch pipeIndex {
	case 0:
		if err := d.writeAddr(PIPE0_RXADDR0, addr); err != nil {
			return err
		}
	case 1:
		if err := d.writeAddr(PIPE1_RXADDR0, addr); err != nil {
			return err
		}
	default:
		// Pipes 2–5: only the LSB (addr[0]) is individually configurable.
		lsbReg := PIPE2_RXADDR0 + pipeIndex - 2
		if err := d.registers.Write(lsbReg, addr[0]); err != nil {
			return err
		}
	}
	d.rxPipeMask |= 1 << pipeIndex
	if err := d.registers.Write(RXPIPE_CFG, d.rxPipeMask); err != nil {
		return err
	}
	if wasRX {
		return d.enterRX()
	}
	return nil
}

// DisableRxAddress disables the given pipe without changing its stored address.
func (d *Driver) DisableRxAddress(pipeIndex uint8) error {
	if pipeIndex > 5 {
		return errors.New("invalid pipe index")
	}
	wasRX := d.inRX
	if err := d.ensureSTB3(); err != nil {
		return err
	}
	d.rxPipeMask &^= 1 << pipeIndex
	if err := d.registers.Write(RXPIPE_CFG, d.rxPipeMask); err != nil {
		return err
	}
	if wasRX {
		return d.enterRX()
	}
	return nil
}

// Send transmits payload to dst. len(payload) must not exceed PayloadLen from config.
// Blocks until TX complete or ~10 ms timeout, then re-enters RX mode.
func (d *Driver) Send(dst Address, payload []byte) error {
	if uint8(len(payload)) > d.payloadLen {
		return ErrPayloadTooLarge
	}
	if err := d.ensureSTB3(); err != nil {
		return err
	}
	if err := d.writeAddr(TXADDR0, dst); err != nil {
		return err
	}
	if err := d.registers.Write(TXPLLEN_CFG, uint8(len(payload))); err != nil {
		return err
	}
	if err := d.registers.WriteBuffer(TRX_FIFO, payload); err != nil {
		return err
	}
	if err := d.registers.Write(RFIRQFLG, IRQ_ALL); err != nil {
		return err
	}
	if err := d.registers.Write(STATE_CFG, STATE_TX); err != nil {
		return err
	}

	txErr := d.pollBit(RFIRQFLG, IRQ_TX, 10*time.Millisecond)

	// Always return to STB3 then RX, regardless of TX outcome.
	_ = d.registers.Write(STATE_CFG, STATE_STB3)
	d.inRX = false
	_ = d.enterRX()

	return txErr
}

// Receive checks for a received packet without blocking.
// Returns (n, true) if a packet was available and copied into buf, (0, false) otherwise.
func (d *Driver) Receive(buf []byte) (n int, ok bool) {
	if !d.inRX {
		if err := d.enterRX(); err != nil {
			return 0, false
		}
	}

	flags, err := d.registers.Read(RFIRQFLG)
	if err != nil || flags&IRQ_RX == 0 {
		return 0, false
	}

	length, err := d.registers.Read(STATUS3)
	if err != nil {
		return 0, false
	}
	if int(length) > len(buf) {
		length = uint8(len(buf))
	}
	if err := d.registers.ReadBuffer(TRX_FIFO, buf[:length]); err != nil {
		return 0, false
	}
	_ = d.registers.Write(RFIRQFLG, IRQ_ALL)
	return int(length), true
}

// DumpState prints key register values over RTT for debugging.
func (d *Driver) DumpState() {
	dump := func(name string, reg uint8) {
		v, err := d.registers.Read(reg)
		if err != nil {
			println(name, "ERR")
		} else {
			println(name, v)
		}
	}
	dump("STATE_CFG ", STATE_CFG)
	dump("WMODE_CFG0", WMODE_CFG0)
	dump("WMODE_CFG1", WMODE_CFG1)
	dump("RF_CHANNEL", RF_CHANNEL_CFG)
	dump("RFIRQFLG  ", RFIRQFLG)
	dump("RFIRQ_CFG ", RFIRQ_CFG)
	dump("STATUS0   ", STATUS0)
	dump("STATUS3   ", STATUS3)
	dump("RXPIPE_CFG", RXPIPE_CFG)
	dump("DATARATE  ", RF_DATARATE_CFG)
}
