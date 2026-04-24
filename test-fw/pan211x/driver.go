package pan211x

import (
	"errors"
	"runtime"
	"time"
)

// Errors returned by Driver methods.
var (
	ErrPayloadTooLarge = errors.New("payload too large")
	ErrTimeout         = errors.New("radio timeout")
	ErrCalibration     = errors.New("calibration failed")
	ErrNoDevice        = errors.New("no device")
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
	DataRate   uint8   // use DATARATE_* constants
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
	if err := d.registers.Write(PAGE_CFG, 0x00); err != nil {
		return err
	}
	// REG_SPI3_REN=1 enables 3-wire SPI reads. Must be set before entering STB3.
	if err := d.registers.Write(SPI_CFG, SPI_CFG_INIT); err != nil {
		return err
	}

	// ── Step 2: Enter STB3 with soft reset ───────────────────────────────────
	if err := d.registers.Write(STATE_CFG, STATE_STB3_INIT); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	if err := d.registers.Write(STATE_CFG, STATE_STB3); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	if err := d.registers.Write(SYS_CFG, SYS_CFG_RESET); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	if err := d.registers.Write(SYS_CFG, SYS_CFG_RELEASE); err != nil {
		return err
	}

	// Verify chip is accessible: SPI_CFG should still read SPI_CFG_INIT after reset.
	v, err := d.registers.Read(SPI_CFG)
	if err != nil {
		return err
	}
	if v != SPI_CFG_INIT {
		println("SPI_CFG readback:", v)
		return ErrNoDevice
	}

	// ── Step 3: Read factory OTP calibration (Page1) ─────────────────────────
	if err := d.registers.Write(PAGE_CFG, 0x01); err != nil {
		return err
	}
	if err := d.registers.Write(P1_OTP_CTL, OTP_CTL_START); err != nil {
		return err
	}
	if err := d.registers.Write(P1_OTP_DATA, OTP_READ_WORD2); err != nil {
		return err
	}
	value2, err := d.registers.Read(P1_OTP_DATA)
	if err != nil {
		return err
	}
	if err := d.registers.Write(P1_OTP_DATA, OTP_READ_WORD4); err != nil {
		return err
	}
	value4, err := d.registers.Read(P1_OTP_DATA)
	if err != nil {
		return err
	}
	if err := d.registers.Write(P1_OTP_CTL, OTP_CTL_STOP); err != nil {
		return err
	}
	println("OTP value2:", value2, "value4:", value4)
	if (value2 & OTP_VALID_MASK) != OTP_VALID_VAL {
		println("OTP check failed, value2&0x0F:", value2&OTP_VALID_MASK)
		return ErrCalibration
	}
	if err := d.registers.Write(P1_PA_TUNE_47, 0x83|((value2>>1)&0x70)); err != nil {
		return err
	}
	calBit := uint8(0)
	if (value2 & OTP_CAL_MASK) == 0 {
		calBit = 1
	}
	if err := d.registers.Write(P1_PA_TUNE_43, 0x10|calBit); err != nil {
		return err
	}

	// ── Step 4: Page1 pre-configuration (SDK normal_tx example) ──────────────
	for _, rw := range []struct{ reg, val uint8 }{
		{P1_RF_TUNE_27, 0xAA}, {P1_RF_TUNE_32, 0x1E}, {P1_RF_TUNE_33, 0x19},
		{P1_RF_TUNE_37, 0x15}, {P1_RF_TUNE_3A, 0x14}, {P1_RF_TUNE_3E, 0xF1},
		{P1_VCO_PA_CTL, 0xA2}, {P1_TX_PWR_AMP, 0x17}, {P1_PA_BIAS, PA_BIAS_9DBM},
		{P1_TX_PWR_CTL, 0x88}, {P1_RF_TUNE_4C, 0x48},
	} {
		if err := d.registers.Write(rw.reg, rw.val); err != nil {
			return err
		}
	}

	// ── Step 5: Page0 RF configuration ───────────────────────────────────────
	if err := d.registers.Write(PAGE_CFG, 0x00); err != nil {
		return err
	}
	// Crystal load capacitor from OTP.
	if err := d.registers.Write(XTAL_CFG, (value4>>4)|0xC0); err != nil {
		return err
	}
	if err := d.registers.Write(SYS_CFG, SYS_CFG_NORMAL); err != nil {
		return err
	}
	// WMODE_CFG0: 2-byte CRC, XN297L mode, whitening enabled, big-endian. Matches SDK 0x89.
	// Whitening is required: long zero runs in payload corrupt receiver CDR without it.
	if err := d.registers.Write(WMODE_CFG0, CRC_2B|WHITEN_EN_BIT|ENDIAN_BIG); err != nil {
		return err
	}
	// WMODE_CFG1: RX_GOON=1, FIFO_128=1 (required in XN297L normal mode per RM p.51), 4-byte addr.
	if err := d.registers.Write(WMODE_CFG1, RX_GOON_BIT|FIFO_128_BIT|ADDR_4B); err != nil {
		return err
	}
	if err := d.registers.Write(RXPLLEN_CFG, d.cfg.PayloadLen); err != nil {
		return err
	}
	if err := d.registers.Write(TXPLLEN_CFG, d.cfg.PayloadLen); err != nil {
		return err
	}
	// Mask only IRQ_MAX_RT (irrelevant with ARC=0). All error IRQs visible in RFIRQFLG.
	if err := d.registers.Write(RFIRQ_CFG, IRQ_MAX_RT); err != nil {
		return err
	}
	if err := d.registers.Write(TXAUTO_CFG, 0x00); err != nil {
		return err
	}
	// TRXMODE_CFG: single TX, continuous RX, pre-sync enabled.
	if err := d.registers.Write(TRXMODE_CFG, TRXMODE_CFG_NORMAL); err != nil {
		return err
	}
	// Whitening seed = WHITEN_DEFAULT (SDK default). Both TX and RX must use same seed.
	if err := d.registers.Write(WHITEN_CFG, WHITEN_DEFAULT); err != nil {
		return err
	}
	// Enable PIPE0 explicitly (default=1, but be explicit after soft reset).
	if err := d.registers.Write(RXPIPE_CFG, PIPE0_EN); err != nil {
		return err
	}

	// Own address → PIPE0_RXADDR hardware RX filter (4 bytes, little-endian).
	a := d.cfg.OwnAddr.Bytes()
	for i, reg := range [4]uint8{
		PIPE0_RXADDR0, PIPE0_RXADDR1,
		PIPE0_RXADDR2, PIPE0_RXADDR3,
	} {
		if err := d.registers.Write(reg, a[i]); err != nil {
			return err
		}
	}

	// Calibration channel, data rate, RF tuning (per SDK ES_Tool V1.2.6; crystal-independent).
	if err := d.registers.Write(RF_CHANNEL_CFG, RF_CH_CAL); err != nil {
		return err
	}
	if err := d.registers.Write(RF_DATARATE_CFG, d.cfg.DataRate); err != nil {
		return err
	}
	for _, rw := range []struct{ reg, val uint8 }{
		{RF_ANA_43, 0x3A}, {RF_ANA_44, RF_ANA_44_9DBM},
		{RF_ANA_55, 0xDD}, {RF_ANA_56, 0xC9}, {RF_ANA_57, 0xB7},
		{RF_ANA_5A, 0x10}, {RF_ANA_5B, 0xFD}, {RF_ANA_5C, 0xE9},
		{RF_ANA_5D, 0xDC}, {RF_ANA_5E, 0x02}, {RF_ANA_5F, 0x06},
		{RF_ANA_60, 0x0E}, {RF_ANA_61, 0x2E},
		{RF_ANA_66, 0x34}, {RF_ANA_68, 0x0D},
		{RF_ANA_6E, 0x20},
	} {
		if err := d.registers.Write(rw.reg, rw.val); err != nil {
			return err
		}
	}

	// ── Step 6: RF calibration (Page1) ───────────────────────────────────────
	if err := d.registers.Write(PAGE_CFG, 0x01); err != nil {
		return err
	}
	// VCO calibration.
	if err := d.registers.Write(P1_CAL_CTL, CAL_VCO); err != nil {
		return err
	}
	if err := d.waitBit(P1_CAL_STATUS_VCO, CAL_VCO_DONE_BIT, 10000); err != nil {
		println("VCO cal timeout")
		return ErrCalibration
	}
	// Thermal calibration: mandatory 55 ms delay.
	if err := d.registers.Write(P1_CAL_CTL, CAL_THERMAL); err != nil {
		return err
	}
	time.Sleep(55 * time.Millisecond)
	// Frequency calibration: requires RX mode.
	if err := d.registers.Write(STATE_CFG, STATE_RX); err != nil {
		return err
	}
	time.Sleep(200 * time.Microsecond)
	if err := d.registers.Write(P1_CAL_CTL, CAL_FREQ); err != nil {
		return err
	}
	if err := d.waitBit(P1_CAL_STATUS_DONE, CAL_DONE_BIT, 10000); err != nil {
		println("freq cal timeout")
		return ErrCalibration
	}
	// Phase calibration 1.
	if err := d.registers.Write(P1_CAL_CTL, CAL_PHASE1); err != nil {
		return err
	}
	if err := d.waitBit(P1_CAL_STATUS_PHASE1, CAL_PHASE1_DONE_BIT, 10000); err != nil {
		println("phase1 cal timeout")
		return ErrCalibration
	}
	// Phase calibration 2.
	if err := d.registers.Write(P1_CAL_CTL, CAL_PHASE2); err != nil {
		return err
	}
	if err := d.waitBit(P1_CAL_STATUS_DONE, CAL_DONE_BIT, 10000); err != nil {
		println("phase2 cal timeout")
		return ErrCalibration
	}
	if err := d.registers.Write(P1_CAL_CTL, CAL_STOP); err != nil {
		return err
	}
	// Back to STB3 and Page0.
	if err := d.registers.Write(STATE_CFG, STATE_STB3); err != nil {
		return err
	}
	if err := d.registers.Write(PAGE_CFG, 0x00); err != nil {
		return err
	}

	// Set operating channel, clear all IRQ, enter RX.
	if err := d.registers.Write(RF_CHANNEL_CFG, d.cfg.RFChannel); err != nil {
		return err
	}
	if err := d.registers.Write(RFIRQFLG, IRQ_ALL); err != nil {
		return err
	}
	return d.registers.Write(STATE_CFG, STATE_RX)
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
	if err := d.registers.Write(STATE_CFG, STATE_STB3); err != nil {
		return err
	}

	// Set TXADDR to destination (4 bytes, little-endian).
	for i := uint8(0); i < 4; i++ {
		if err := d.registers.Write(TXADDR0+i, dst[i]); err != nil {
			return err
		}
	}

	// Update TX payload length in case it differs from the init value.
	if err := d.registers.Write(TXPLLEN_CFG, uint8(len(payload))); err != nil {
		return err
	}

	// Write payload into TX FIFO.
	if err := d.registers.WriteBuffer(TRX_FIFO, payload); err != nil {
		return err
	}

	// Clear any stale IRQ flags, then enter TX.
	if err := d.registers.Write(RFIRQFLG, IRQ_ALL); err != nil {
		return err
	}
	if err := d.registers.Write(STATE_CFG, STATE_TX); err != nil {
		return err
	}

	// Poll for TX-complete IRQ. With TxMode=TX_SINGLE, chip returns to STB3 after TX.
	for i := 0; i < 5000; i++ {
		flags, err := d.registers.Read(RFIRQFLG)
		if err != nil {
			_ = d.enterRX()
			return err
		}
		if flags&IRQ_TX != 0 {
			_ = d.registers.Write(RFIRQFLG, IRQ_ALL)
			return d.enterRX()
		}
		runtime.Gosched()
	}

	state, _ := d.registers.Read(STATE_CFG)
	irqFlags, _ := d.registers.Read(RFIRQFLG)
	println("TX timeout STATE_CFG:", state, "RFIRQFLG:", irqFlags)
	_ = d.enterRX()
	return ErrTimeout
}

// Receive checks whether a packet has been received. Non-blocking.
// Fixed-length mode (EnDPL=0): length == PayloadLen == len(buf).
// RX_GOON=1 means the chip stays in RX automatically after each received packet.
func (d *Driver) Receive(buf []byte) (n int, ok bool) {
	flags, err := d.registers.Read(RFIRQFLG)
	if err != nil || flags&IRQ_RX == 0 {
		return 0, false
	}
	println("RX IRQ fired, flags:", flags)
	if err := d.registers.ReadBuffer(TRX_FIFO, buf); err != nil {
		_ = d.registers.Write(RFIRQFLG, IRQ_ALL)
		return 0, false
	}
	_ = d.registers.Write(RFIRQFLG, IRQ_RX)
	return len(buf), true
}

// DumpState prints key register values to RTT for debugging.
// Always operates on page0 (restores page0 after reading).
func (d *Driver) DumpState() {
	d.registers.Write(PAGE_CFG, 0x01)
	pwrAmp, _ := d.registers.Read(P1_TX_PWR_AMP)
	pwrCtl, _ := d.registers.Read(P1_TX_PWR_CTL)
	paBias, _ := d.registers.Read(P1_PA_BIAS)
	d.registers.Write(PAGE_CFG, 0x00)
	state, _ := d.registers.Read(STATE_CFG)
	irq, _ := d.registers.Read(RFIRQFLG)
	irqmask, _ := d.registers.Read(RFIRQ_CFG)
	wmode0, _ := d.registers.Read(WMODE_CFG0)
	wmode1, _ := d.registers.Read(WMODE_CFG1)
	whiten, _ := d.registers.Read(WHITEN_CFG)
	pktExt, _ := d.registers.Read(PKT_EXT_CFG)
	trxmode, _ := d.registers.Read(TRXMODE_CFG)
	txauto, _ := d.registers.Read(TXAUTO_CFG)
	ch, _ := d.registers.Read(RF_CHANNEL_CFG)
	dr, _ := d.registers.Read(RF_DATARATE_CFG)
	rxlen, _ := d.registers.Read(RXPLLEN_CFG)
	a0, _ := d.registers.Read(PIPE0_RXADDR0)
	a1, _ := d.registers.Read(PIPE0_RXADDR1)
	a2, _ := d.registers.Read(PIPE0_RXADDR2)
	a3, _ := d.registers.Read(PIPE0_RXADDR3)
	t0, _ := d.registers.Read(TXADDR0)
	t1, _ := d.registers.Read(TXADDR1)
	t2, _ := d.registers.Read(TXADDR2)
	t3, _ := d.registers.Read(TXADDR3)
	rssiL, _ := d.registers.Read(RT_RSSI_L)
	rssiH, _ := d.registers.Read(RT_RSSI_H)
	println("--- STATE_CFG:", state)
	println("  OPERATE_MODE:", state&0x07, " (4=STB3 5=TX 6=RX)")
	println("--- RFIRQFLG:", irq, " RFIRQ_CFG(mask):", irqmask)
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
	println("  WHITEN_CFG:", whiten, " (seed=[6:0], skip_addr=[7])")
	println("  PKT_EXT_CFG:", pktExt, " TRXMODE_CFG:", trxmode, " TXAUTO_CFG:", txauto)
	println("  RXADDR:", a0, a1, a2, a3)
	println("  TXADDR:", t0, t1, t2, t3)
	println("  RT_RSSI_L:", rssiL, " RT_RSSI_H:", rssiH)
	println("--- P1 PA (9dBm: PWR_AMP=23 PWR_CTL=136 PA_BIAS=176)")
	println("  P1_TX_PWR_AMP:", pwrAmp, " P1_TX_PWR_CTL:", pwrCtl, " P1_PA_BIAS:", paBias)
}

// enterRX enters RX from STB3. Used after TX completes.
func (d *Driver) enterRX() error {
	if err := d.registers.Write(STATE_CFG, STATE_STB3); err != nil {
		return err
	}
	if err := d.registers.Write(RFIRQFLG, IRQ_ALL); err != nil {
		return err
	}
	return d.registers.Write(STATE_CFG, STATE_RX)
}
