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
	ErrInvalidChannel  = errors.New("invalid channel")
)

type Address [5]byte

type BitRate uint8

const (
	BitRate250Kbps BitRate = 0
	BitRate1Mbps   BitRate = 1
	BitRate2Mbps   BitRate = 2
)

const maxChannel = 83

// Registers abstracts the physical bus (I2C or SPI) for register access.
type Registers interface {
	Read(reg uint8) (uint8, error)
	Write(reg uint8, value uint8) error
	WriteBuffer(reg uint8, data []byte) error
	ReadBuffer(reg uint8, buf []byte) error
}

type SerialInterface uint8

const (
	SerialInterfaceSPI3W SerialInterface = 0
	SerialInterfaceSPI4W SerialInterface = 1
	SerialInterfaceI2C   SerialInterface = 2
)

type ConfigXN297L struct {
	BitRate         BitRate
	PayloadLen      uint8
	SerialInterface SerialInterface
}

type Driver struct {
	registers  Registers
	payloadLen uint8
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
	return d.registers.Write(STATE_CFG, STATE_RX)
}

func (d *Driver) ensureSTB3() error {
	return d.registers.Write(STATE_CFG, STATE_STB3)
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
	// Soft reset clears SPI_CFG to its default, which has REG_SPI3_REN=0 (reads disabled).
	// Must re-enable 3-wire SPI reads before any register read operation.
	if cfg.SerialInterface == SerialInterfaceSPI3W {
		if err := r.Write(SPI_CFG, SPI_CFG_INIT_3W); err != nil {
			return err
		}
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

	// Wrap up: stop FSM, return to Page 0, enter RX.
	if err := r.Write(P1_CAL_CTL, CAL_STOP); err != nil {
		return err
	}
	if err := r.Write(PAGE_CFG, 0x00); err != nil {
		return err
	}
	// RF_CHANNEL_CFG is still RF_CH_CAL; caller must call SetChannel before use.
	return d.enterRX()
}

// SetChannel sets the RF channel. ch = frequency_MHz − 2400 (valid 0–83).
func (d *Driver) SetChannel(channel uint8) error {
	if channel > maxChannel {
		return ErrInvalidChannel
	}
	if err := d.ensureSTB3(); err != nil {
		return err
	}
	if err := d.registers.Write(RF_CHANNEL_CFG, channel); err != nil {
		return err
	}
	return d.enterRX()
}

// EnableRxAddress sets the receive address for pipe pipeIndex (0–5) and enables the pipe.
// Pipes 0 and 1 use the full 5-byte addr. Pipes 2–5 use only addr[0] (LSB);
// their upper 4 bytes are shared with pipe 1 and must be set via pipe 1 first.
func (d *Driver) EnableRxAddress(pipeIndex uint8, addr Address) error {
	if pipeIndex > 5 {
		return errors.New("invalid pipe index")
	}
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
	mask, err := d.registers.Read(RXPIPE_CFG)
	if err != nil {
		return err
	}
	if err := d.registers.Write(RXPIPE_CFG, mask|(1<<pipeIndex)); err != nil {
		return err
	}
	return d.enterRX()
}

// DisableRxAddress disables the given pipe without changing its stored address.
func (d *Driver) DisableRxAddress(pipeIndex uint8) error {
	if pipeIndex > 5 {
		return errors.New("invalid pipe index")
	}
	if err := d.ensureSTB3(); err != nil {
		return err
	}
	mask, err := d.registers.Read(RXPIPE_CFG)
	if err != nil {
		return err
	}
	if err := d.registers.Write(RXPIPE_CFG, mask&^(1<<pipeIndex)); err != nil {
		return err
	}
	return d.enterRX()
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

	// Always re-enter RX regardless of TX outcome.
	_ = d.enterRX()

	return txErr
}

// Receive checks for a received packet without blocking.
// Returns (n, true) if a packet was available and copied into buf, (0, false) otherwise.
func (d *Driver) Receive(buf []byte) (n int, ok bool) {

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

// DumpState prints key register values with decoded field meanings for debugging.
func (d *Driver) DumpState() {
	r := d.registers
	println("--- PAN211x ---")

	rd := func(reg uint8) (uint8, bool) {
		v, err := r.Read(reg)
		if err != nil {
			return 0, false
		}
		return v, true
	}

	if v, ok := rd(STATE_CFG); ok {
		s := "unknown"
		switch v {
		case STATE_STB3:
			s = "STB3"
		case STATE_TX:
			s = "TX"
		case STATE_RX:
			s = "RX"
		case STATE_SLEEP:
			s = "SLEEP"
		case STATE_STB3_INIT:
			s = "STB3_INIT"
		}
		println("STATE     :", s)
	}

	if v, ok := rd(SPI_CFG); ok {
		en := "0"
		if v&0x80 != 0 {
			en = "1"
		}
		println("SPI_CFG   : 3wire_ren=" + en)
	}

	if v, ok := rd(WMODE_CFG0); ok {
		crc := "OFF"
		switch v & 0xC0 {
		case CRC_1B:
			crc = "1B"
		case CRC_2B:
			crc = "2B"
		case CRC_3B:
			crc = "3B"
		}
		mode := "XN297L"
		switch v & 0x30 {
		case WORK_MODE_FS01:
			mode = "FS01"
		case WORK_MODE_FS32:
			mode = "FS32/BLE"
		}
		whiten, endian := "0", "LE"
		if v&WHITEN_EN_BIT != 0 {
			whiten = "1"
		}
		if v&ENDIAN_BIG != 0 {
			endian = "BE"
		}
		println("WMODE_CFG0: crc=" + crc + " mode=" + mode + " whiten=" + whiten + " endian=" + endian)
	}

	if v, ok := rd(WMODE_CFG1); ok {
		rxgoon, fifo, dpy, enh := "0", "64B", "0", "0"
		if v&RX_GOON_BIT != 0 {
			rxgoon = "1"
		}
		if v&FIFO_128_BIT != 0 {
			fifo = "128B"
		}
		if v&DPY_EN_BIT != 0 {
			dpy = "1"
		}
		if v&ENHANCE_BIT != 0 {
			enh = "1"
		}
		addr := "?"
		switch v & 0x03 {
		case ADDR_2B:
			addr = "2B"
		case ADDR_3B:
			addr = "3B"
		case ADDR_4B:
			addr = "4B"
		case ADDR_5B:
			addr = "5B"
		}
		println("WMODE_CFG1: rx_goon=" + rxgoon + " fifo=" + fifo + " dpy=" + dpy + " enh=" + enh + " addr=" + addr)
	}

	if v, ok := rd(RXPIPE_CFG); ok {
		pipes := ""
		for i := uint8(0); i < 6; i++ {
			if v>>i&1 != 0 {
				pipes += string([]byte{'0' + i})
			}
		}
		if pipes == "" {
			pipes = "(none)"
		}
		println("RXPIPE_CFG: enabled=" + pipes)
	}

	if v, ok := rd(RF_CHANNEL_CFG); ok {
		println("CHANNEL   : ch=", v, "/ freq=", 2400+int(v), "MHz")
	}

	{
		rx, ok1 := rd(RXPLLEN_CFG)
		tx, ok2 := rd(TXPLLEN_CFG)
		if ok1 && ok2 {
			println("PAYLOAD   : rx=", rx, "tx=", tx, "bytes")
		}
	}

	if v, ok := rd(RF_DATARATE_CFG); ok {
		rate := "1Mbps"
		switch v & DATARATE_BW_MASK {
		case DATARATE_BW_2MBPS:
			rate = "2Mbps"
		case DATARATE_BW_250KBPS:
			rate = "250kbps"
		}
		println("DATARATE  :", rate)
	}

	if v, ok := rd(TRXMODE_CFG); ok {
		tx := "SINGLE"
		if v&TX_CONTINUOUS_BIT != 0 {
			tx = "CW"
		}
		rx := "SINGLE"
		switch v & 0x60 {
		case RX_TIMEOUT_BIT:
			rx = "TIMEOUT"
		case RX_CONTINUOUS_BIT:
			rx = "CONT"
		}
		presync := "0"
		if v&PRE_SYNC_EN_BIT != 0 {
			presync = "1"
		}
		println("TRXMODE   : tx=" + tx + " rx=" + rx + " presync=" + presync)
	}

	if v, ok := rd(RFIRQFLG); ok {
		flags := irqNames(v)
		println("RFIRQFLG  : set=" + flags)
	}

	if v, ok := rd(RFIRQ_CFG); ok {
		masked := irqNames(v)
		println("RFIRQ_CFG : masked=" + masked)
	}

	if v, ok := rd(STATUS0); ok {
		pipe := (v & STATUS0_PIPE_MASK) >> STATUS0_PIPE_SHIFT
		cierr := "0"
		if v&STATUS0_CI_ERR_BIT != 0 {
			cierr = "1"
		}
		extra := ""
		if pipe == STATUS0_PIPE_EMPTY>>STATUS0_PIPE_SHIFT {
			extra = " (FIFO empty)"
		}
		println("STATUS0   : pipe=" + string([]byte{'0' + pipe}) + " ci_err=" + cierr + extra)
	}

	if v, ok := rd(STATUS3); ok {
		println("STATUS3   : rxlen=", v)
	}
}

func irqNames(v uint8) string {
	s := ""
	if v&IRQ_TX != 0 {
		s += " TX"
	}
	if v&IRQ_MAX_RT != 0 {
		s += " MAX_RT"
	}
	if v&IRQ_ADDR_ERR != 0 {
		s += " ADDR_ERR"
	}
	if v&IRQ_CRC_ERR != 0 {
		s += " CRC_ERR"
	}
	if v&IRQ_LEN_ERR != 0 {
		s += " LEN_ERR"
	}
	if v&IRQ_PID_ERR != 0 {
		s += " PID_ERR"
	}
	if v&IRQ_RX_TIMEOUT != 0 {
		s += " RX_TO"
	}
	if v&IRQ_RX != 0 {
		s += " RX"
	}
	if s == "" {
		return "(none)"
	}
	return s[1:] // trim leading space
}
