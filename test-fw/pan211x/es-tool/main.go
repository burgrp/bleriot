// pan211x-config: generates PAN211x BLE register initialization tables.
//
// Ports the ES_Tool SetupConfig() logic (modified_genconfig.py) to Go.
// Outputs a Go source fragment with P1 and P0 register init tables that
// can be embedded into driver.go Init().
//
// Usage:
//
//	go run ./es-tool --channel 37 --xtal 16 --power 9 --iface i2c
//
// The generated tables assume:
//   - BLE ADV_NONCONN_IND TX beacon (no ACK/RX, standard BLE advertising)
//   - 1 Mbps data rate, no AGC, low RX gain
//   - Payload = AdvA(6) + AdvData; header (PDU type + length) is auto-inserted
package main

import (
	"flag"
	"fmt"
	"math/bits"
	"os"
	"strings"
)

// ── register simulation ───────────────────────────────────────────────────────

type regState struct {
	p0 [128]uint8
	p1 [128]uint8
}

func newRegState() *regState {
	rs := &regState{}
	// DefaultRegPage0 from genconfig_reg.py (addr → value, only non-zero entries listed)
	for _, kv := range [][2]uint8{
		{0x02, 116}, {0x03, 2}, {0x04, 115}, {0x05, 136}, {0x06, 5},
		{0x07, 73}, {0x08, 131},
		{0x0F, 204}, {0x10, 204}, {0x11, 204}, {0x12, 204}, {0x13, 204},
		{0x14, 204}, {0x15, 204}, {0x16, 204}, {0x17, 204}, {0x18, 204},
		{0x1A, 127}, {0x1F, 1},
		{0x20, 204}, {0x21, 204}, {0x22, 204}, {0x23, 204}, {0x24, 204},
		{0x25, 204}, {0x26, 204}, {0x27, 204}, {0x28, 204},
		{0x29, 3}, {0x2A, 1}, {0x2B, 208}, {0x2C, 7},
		{0x35, 7},
		{0x36, 69}, {0x37, 96}, {0x38, 16}, {0x39, 78}, {0x3A, 87},
		{0x3E, 30},
		{0x43, 50}, {0x44, 124},
		{0x47, 63}, {0x48, 2},
		{0x4A, 64}, {0x4B, 64},
		{0x4D, 129}, {0x4E, 63}, {0x4F, 32},
		{0x50, 48}, {0x51, 32}, {0x52, 48}, {0x53, 17},
		{0x55, 214}, {0x56, 194}, {0x57, 176}, {0x58, 168}, {0x59, 6},
		{0x5A, 6}, {0x5B, 242}, {0x5C, 222}, {0x5D, 205},
		{0x5E, 3}, {0x5F, 7}, {0x60, 15}, {0x61, 63},
		{0x62, 172}, {0x63, 186}, {0x64, 32}, {0x65, 16}, {0x66, 100},
		{0x67, 17}, {0x68, 25}, {0x69, 40},
		{0x6E, 64},
		{0x6F, 63},
		{0x70, 36}, {0x71, 94},
	} {
		rs.p0[kv[0]] = kv[1]
	}
	// DefaultRegPage1 from genconfig_reg.py
	for _, kv := range [][2]uint8{
		{0x05, 1}, {0x06, 150}, {0x07, 117}, {0x08, 22}, {0x09, 134},
		{0x0A, 200}, {0x0B, 16}, {0x0C, 80}, {0x0D, 9}, {0x0E, 101},
		{0x0F, 143}, {0x10, 60}, {0x11, 20}, {0x12, 178}, {0x13, 5},
		{0x14, 66}, {0x15, 89}, {0x16, 136}, {0x17, 130}, {0x18, 46},
		{0x19, 7}, {0x1A, 16}, {0x1C, 192}, {0x1D, 131}, {0x1E, 68},
		{0x1F, 34}, {0x20, 136}, {0x21, 136}, {0x22, 136}, {0x23, 112},
		{0x24, 16}, {0x25, 112}, {0x26, 16}, {0x27, 10},
		{0x29, 8}, {0x2A, 5}, {0x2D, 32}, {0x2E, 210}, {0x2F, 5},
		{0x30, 79}, {0x31, 1}, {0x32, 24}, {0x33, 27}, {0x34, 160},
		{0x35, 32}, {0x36, 11}, {0x37, 19}, {0x38, 28}, {0x39, 187},
		{0x3A, 19}, {0x3B, 106}, {0x3C, 23}, {0x3D, 3}, {0x3E, 245},
		{0x3F, 212}, {0x40, 40}, {0x41, 145}, {0x42, 164}, {0x43, 18},
		{0x44, 204}, {0x45, 224}, {0x46, 189}, {0x47, 179}, {0x48, 136},
		{0x49, 196}, {0x4A, 5}, {0x4B, 160}, {0x4C, 120},
		{0x50, 142},
		{0x5C, 240}, {0x5D, 32}, {0x5E, 100}, {0x5F, 100}, {0x60, 200},
		{0x67, 226}, {0x68, 69}, {0x69, 54}, {0x6A, 156},
	} {
		rs.p1[kv[0]] = kv[1]
	}
	return rs
}

func trailingZeros8(v uint8) int { return bits.TrailingZeros8(v) }

func (rs *regState) writeP0(addr, val uint8)                  { rs.p0[addr] = val }
func (rs *regState) writeP1(addr, val uint8)                  { rs.p1[addr] = val }
func (rs *regState) readP0(addr uint8) uint8                  { return rs.p0[addr] }
func (rs *regState) readP1(addr uint8) uint8                  { return rs.p1[addr] }

func (rs *regState) bitsP0(addr, bitval, mask uint8) {
	shift := trailingZeros8(mask)
	rs.p0[addr] = (rs.p0[addr] &^ mask) | ((bitval<<shift)&mask)
}

func (rs *regState) bitsP1(addr, bitval, mask uint8) {
	shift := trailingZeros8(mask)
	rs.p1[addr] = (rs.p1[addr] &^ mask) | ((bitval<<shift)&mask)
}

// ── ES_Tool SetupConfig BLE TX beacon ────────────────────────────────────────

// BLEChannel identifies a BLE advertising channel.
type BLEChannel struct {
	index   int    // channel index (37, 38, 39)
	rfCh    uint8  // raw RF_CH value for PAN211x
	whInit  uint8  // whitening LFSR seed (SCR_CFG bits[6:0])
}

var bleChannels = map[int]BLEChannel{
	37: {37, 2, 83},
	38: {38, 26, 51},
	39: {39, 80, 115},
}

// TxPower describes one TX power level and the register values it requires.
type TxPower struct {
	dBm         int
	poutRes     uint8 // P1[0x3C] bits[2:0]: TXPA_POUT_RES
	tpaModePA   uint8 // P0[0x43] bits[5:4]: TXPA_MODE_SEL field value
	poutCrnt    uint8 // P0[0x44] bits[7:4]: TXPA_POUT_CRNT field value
	ldoSel      uint8 // P0[0x44] bits[3:0]: TXPA_LDO_SEL field value
	mtchC2TX    uint8 // P1[0x46] bit0: RFMTCHNTWK_C2VAL_TX
	mtchC1TX    uint8 // P1[0x46] bits[3:2]: RFMTCHNTWK_C1VAL_TX
	ldoPA       uint8 // P1[0x3C] bit3: LDO_PA_BYPASS_EN (1 only for 11dBm)
	ldoRffe     uint8 // P1[0x48] bits[3:0]: LDO_RFFE_TRIM (8 normal, 12 lowpwr, 15 max)
	codeOffset  uint8 // P1[0x27]: BLE 1Mbps code offset
}

// sop8PowerTable ports sop8_power_table from modified_genconfig.py (BLE entries).
// For BLE 1Mbps the CodeOffset is always 0xAA (same as XN297 1Mbps).
var sop8PowerTable = []TxPower{
	{11, 7, 3, 15, 12, 0, 0, 1, 15, 0xAA},
	{9, 7, 3, 8, 12, 0, 0, 0, 8, 0xAA},
	{8, 7, 3, 8, 6, 0, 0, 0, 8, 0xAA},
	{7, 7, 3, 8, 3, 0, 0, 0, 8, 0xAA},
	{6, 7, 3, 8, 4, 0, 1, 0, 8, 0xAA},
	{5, 7, 3, 8, 2, 0, 1, 0, 8, 0xAA},
	{4, 7, 3, 8, 0, 0, 1, 0, 8, 0xAA},
	{3, 7, 3, 8, 0, 0, 2, 0, 8, 0xAA},
	{2, 3, 3, 8, 2, 0, 3, 0, 8, 0xAA},
	{1, 3, 3, 8, 0, 0, 3, 0, 8, 0xAA},
	{0, 3, 3, 8, 4, 1, 3, 0, 8, 0xAA},
	{-1, 4, 3, 8, 0, 1, 3, 0, 8, 0xAA},
	{-2, 7, 1, 15, 15, 0, 1, 0, 8, 0xAA},
	{-5, 7, 1, 15, 15, 1, 3, 0, 8, 0xAA},
	{-7, 3, 1, 8, 8, 1, 3, 0, 8, 0xAA},
	{-8, 3, 1, 8, 4, 1, 1, 0, 8, 0xAA},
	{-10, 3, 1, 8, 0, 1, 0, 0, 8, 0xAA},
	{-11, 3, 1, 6, 0, 1, 0, 0, 8, 0xAA},
	{-12, 3, 1, 5, 0, 1, 0, 0, 8, 0xAA},
	{-14, 3, 1, 4, 0, 1, 0, 0, 8, 0xAA},
	{-16, 3, 1, 3, 0, 1, 0, 0, 8, 0xAA},
	{-19, 3, 1, 2, 0, 1, 0, 0, 8, 0xAA},
	{-23, 3, 1, 1, 0, 1, 0, 0, 8, 0xAA},
	{-25, 2, 1, 1, 0, 1, 0, 0, 8, 0xAA},
	{-28, 1, 1, 1, 8, 1, 0, 0, 8, 0xAA},
	{-33, 3, 1, 0, 0, 1, 0, 0, 8, 0xAA},
	{-37, 0, 1, 0, 0, 0, 0, 0, 8, 0xAA},
	{-40, 0, 1, 0, 0, 1, 0, 0, 8, 0xAA},
}

func findPower(dBm int) (TxPower, bool) {
	for _, p := range sop8PowerTable {
		if p.dBm == dBm {
			return p, true
		}
	}
	return TxPower{}, false
}

// setupConfigBLE simulates ES_Tool SetupConfig() for BLE TX beacon mode.
// Parameters match the BLE advertising channel preset in home_table.py.
// payloadLen = AdvA(6) + AdvData; header is auto-inserted by chip (PKT_EXT_CFG).
func setupConfigBLE(rs *regState, ch BLEChannel, xtalMHz int, pwr TxPower,
	payloadLen uint8, useI2C bool) {

	// ── WriteRecommendedRegs (EN_AGC=0) ──────────────────────────────────────
	rs.writeP1(0x27, 0xCA) // initial CodeOffset (overwritten by SetTxPower)
	rs.writeP1(0x3E, 0xF1) // recommended RF tune
	// non-AGC path:
	rs.writeP0(0x4E, 0x7E) // RF_AGC_GAIN_CFG: GAIN_SEL=1, GAIN_OVRD=62
	rs.writeP0(0x57, 0xDD) // RSSI threshold 3
	rs.writeP0(0x5A, 0xCD) // RF_RSSI_FIX0
	rs.writeP0(0x5B, 0xCD) // RF_RSSI_FIX1
	rs.writeP0(0x5C, 0xCD) // RF_RSSI_FIX2
	rs.writeP0(0x61, 0x2E) // RF_GAIN_WORD3 (placeholder; overridden by SetRxGain)

	// ── SetPredefinedRegs ─────────────────────────────────────────────────────
	rs.writeP0(0x02, 0x74) // STATE_STB3 (also done during driver init sequence)
	if xtalMHz == 16 {
		rs.writeP0(0x37, 0xE0) // RF_OSC_CFG: OSC_SEL=1 (16 MHz)
	}
	// I2C mode: no SPI_CFG / SYS_CFG writes

	// ── P1[0x4C] bit4 clear (early) ──────────────────────────────────────────
	rs.bitsP1(0x4C, 0, 0x10) // FSYNXO_STARTUP_FAST = 0

	// ── SetChipMode(BLE) ──────────────────────────────────────────────────────
	rs.bitsP0(0x07, 1, 0x20) // WMODE_CFG0: CHIP_MODE=1
	rs.bitsP0(0x07, 1, 0x10) // WMODE_CFG0: NORDIC_ENHANCE=1
	// SetEndian(LITTLE):
	rs.bitsP0(0x6F, 1, 0x10) // MISC_CFG: PID_LOW_SEL=1 (little-endian marker)
	rs.bitsP0(0x07, 0, 0x01) // WMODE_CFG0: ENDIAN=0 (little)
	// CrcSkipAddr(True):
	rs.bitsP0(0x07, 1, 0x04) // WMODE_CFG0: ACCADDR_CRC_DIS=1
	// WhiteSkipAddr(True):
	rs.bitsP0(0x1A, 1, 0x80) // SCR_CFG: ACCADDR_SCR_DIS=1
	rs.bitsP0(0x08, 1, 0x80) // WMODE_CFG1: RX_GOON=1

	// ── SetChannel ───────────────────────────────────────────────────────────
	rs.writeP0(0x39, ch.rfCh) // RF_CHANNEL_CFG

	// ── SetDataRate(BLE, 1Mbps) ──────────────────────────────────────────────
	rs.bitsP0(0x36, 0, 0x30)  // BW_MODE=0 (1 Mbps)
	// DRModConfig(BLE, 1Mbps):
	rs.bitsP1(0x49, 1, 0x80)  // TX_DAC_GC=1 (high gain)
	rs.bitsP0(0x43, 0, 0x04)  // FSYNVCO_TXCTK=0
	rs.bitsP0(0x43, 2, 0x03)  // RXFLTR_IF=2
	rs.bitsP1(0x3A, 0, 0xC0)  // RXFLTRTUNE_OFST=0
	rs.bitsP1(0x49, 4, 0x70)  // TX_DAC_ISEL=4
	rs.bitsP1(0x4C, 0, 0x20)  // TX_DAC_BW=0 (narrow for 1 Mbps)
	// TxDemodConfig(BLE, 1Mbps, index=12):
	rs.bitsP1(0x32, 16, 0x1F) // TxDemod table[0]: bits[4:0]=16
	rs.bitsP1(0x33, 27, 0x3F) // TxDemod table[1]: bits[5:0]=27
	// RxDemodConfig(BLE, 1Mbps, S2S8=0, column=0):
	rs.bitsP0(0x38, 16, 0x1F) // P0[0x38] bits[4:0]=16
	rs.bitsP0(0x38, 0, 0x60)  // P0[0x38] bits[6:5]=0
	rs.bitsP0(0x37, 96, 0x7F) // P0[0x37] bits[6:0]=96 (preserves OSC_SEL bit7)
	rs.bitsP0(0x36, 0, 0x80)  // DEMOD_DFE_EN_OVRD=0
	rs.bitsP0(0x36, 1, 0x40)  // DEMOD_EN_DFE=1
	rs.bitsP0(0x36, 5, 0x0F)  // DEMOD_COEFF_UNFILT=5
	rs.bitsP1(0x07, 7, 0xF0)  // DEMOD_UNFILT_THRESH=7
	rs.bitsP1(0x07, 5, 0x0F)  // DEMOD_FFE_THRESH=5
	rs.bitsP1(0x0D, 9, 0x3F)  // DEMOD_DFE_COEFF=9
	rs.bitsP1(0x0F, 15, 0x1F) // DEMOD_DFE_FF_COEFF=15
	rs.bitsP1(0x0E, 0, 0x80)  // DEMOD_AGG_CDR_OVRD=0
	rs.bitsP1(0x0E, 1, 0x40)  // DEMOD_AGGRESSIVE_CDR=1
	rs.bitsP1(0x15, 1, 0x40)  // I_LONG_RANGE_CDR_EN=1
	rs.bitsP1(0x5C, 1, 0x80)  // I_CDR_INIT_EN=1
	rs.bitsP1(0x5D, 0, 0x40)  // I_SAMP_MID_ACTION_EN=0
	rs.bitsP1(0x0A, 1, 0x80)  // CLR_OFFSET_EST_EN=1

	// ── SetTxPayloadLen / SetRxPayloadLen ─────────────────────────────────────
	rs.writeP0(0x0A, payloadLen) // TXPLLEN_CFG
	rs.writeP0(0x09, payloadLen) // RXPLLEN_CFG

	// ── RxLengthLimit(True) ───────────────────────────────────────────────────
	rs.bitsP0(0x19, 1, 0x80) // PKT_EXT_CFG: W_RX_MAX_CTRL_EN=1

	// ── SetTxPower ────────────────────────────────────────────────────────────
	rs.bitsP1(0x3C, pwr.poutRes, 0x07)  // TXPA_POUT_RES
	rs.bitsP0(0x43, pwr.tpaModePA, 0x30) // TXPA_MODE_SEL
	rs.bitsP0(0x44, pwr.poutCrnt, 0xF0) // TXPA_POUT_CRNT
	rs.bitsP0(0x44, pwr.ldoSel, 0x0F)   // TXPA_LDO_SEL
	rs.bitsP1(0x46, pwr.mtchC2TX, 0x01) // RFMTCHNTWK_C2VAL_TX
	rs.bitsP1(0x46, pwr.mtchC1TX, 0x0C) // RFMTCHNTWK_C1VAL_TX
	rs.writeP1(0x27, pwr.codeOffset)    // CodeOffset (overrides WriteRecommendedRegs)
	rs.bitsP1(0x48, pwr.ldoRffe, 0x0F)  // LDO_RFFE_TRIM
	rs.bitsP1(0x3C, pwr.ldoPA, 0x08)    // LDO_PA_BYPASS_EN

	// ── SetTxMode(SINGLE) / SetRxMode(CONTINUOUS) ────────────────────────────
	rs.bitsP0(0x2A, 0, 0x80) // TRXMODE_CFG: TX_SINGLE
	rs.bitsP0(0x2A, 2, 0x60) // TRXMODE_CFG: RX_CONTINUOUS

	// ── BLE-specific setup ────────────────────────────────────────────────────
	rs.bitsP0(0x08, 1, 0x10) // WMODE_CFG1: DPY_EN=1 (dynamic payload)
	rs.bitsP0(0x07, 1, 0x08) // WMODE_CFG0: WHITEN_EN=1
	rs.bitsP0(0x07, 3, 0xC0) // WMODE_CFG0: CRC_3byte
	rs.bitsP0(0x08, 0, 0x0C) // WMODE_CFG1: WorkMode=NORMAL (ENHANCE=0, NORMAL_M1=0)
	rs.bitsP0(0x08, 2, 0x03) // WMODE_CFG1: ADDR_4B (AddrWidth 4-2=2)
	// Pipe 0 RX address (BLE default: [0xC0, 0xCC, 0xCC, 0xCC]):
	rs.writeP0(0x0F, 0xC0) // PIPE0_RXADDR0; bytes 1-3 stay at 0xCC default
	// Enable all 6 pipes (all RxAddr enabled in BLE default config):
	rs.bitsP0(0x1F, 1, 0x01) // pipe 0
	rs.bitsP0(0x1F, 1, 0x02) // pipe 1
	rs.bitsP0(0x1F, 1, 0x04) // pipe 2
	rs.bitsP0(0x1F, 1, 0x08) // pipe 3
	rs.bitsP0(0x1F, 1, 0x10) // pipe 4
	rs.bitsP0(0x1F, 1, 0x20) // pipe 5
	// Pipe 1 RX address (4 bytes in BLE 4B mode: [0xC1, 0xCC, 0xCC, 0xCC]):
	rs.writeP0(0x20, 0xC1) // PIPE1_RXADDR0; bytes 1-3 stay at 0xCC default
	// Pipes 2-5 RX addresses (1 byte each, LSB only; MSBs shared with pipe 1):
	rs.writeP0(0x25, 0xC2) // PIPE2_RXADDR0
	rs.writeP0(0x26, 0xC3) // PIPE3_RXADDR0
	rs.writeP0(0x27, 0xC4) // PIPE4_RXADDR0
	rs.writeP0(0x28, 0xC5) // PIPE5_RXADDR0
	// SetS2S8Mode(DISABLE):
	rs.bitsP0(0x2A, 1, 0x01)  // TRXMODE_CFG: PRE_SYNC_EN=1
	rs.bitsP0(0x19, 0, 0x03)  // PKT_EXT_CFG: PRI_CI_MODE=0
	rs.bitsP0(0x19, 0, 0x08)  // PKT_EXT_CFG: PRI_TX_FEC=0
	rs.bitsP0(0x19, 0, 0x04)  // PKT_EXT_CFG: PRI_RX_FEC=0
	rs.bitsP1(0x0B, 0, 0x20)  // DEMOD_BLE_LONG_RANGE=0
	// WriteNordicPktHeader(0x42, 0x00, payloadLen):
	rs.writeP0(0x1B, 0x42)      // TXHDR0_CFG: ADV_NONCONN_IND | TxAdd=1
	rs.writeP0(0x1C, 0x00)      // TXHDR1_CFG: length (auto)
	rs.writeP0(0x0A, payloadLen) // TXPLLEN_CFG (redundant write)
	// SetNordicPktHeader(True, 2):
	rs.bitsP0(0x19, 1, 0x40)   // PKT_EXT_CFG: HDR_LEN_EXIST=1
	rs.bitsP0(0x19, 2, 0x30)   // PKT_EXT_CFG: HDR_LEN_NUMB=2
	// BLE whitelist (WLIST1..5, start=0, 5 bytes of 0xCC default):
	rs.bitsP0(0x35, 0, 0x3F)   // BLEMATCHSTART_CFG: start_byte=0
	rs.writeP0(0x30, 0xCC)     // WLIST1
	rs.writeP0(0x31, 0xCC)     // WLIST2
	rs.writeP0(0x32, 0xCC)     // WLIST3
	rs.writeP0(0x33, 0xCC)     // WLIST4
	rs.writeP0(0x34, 0xCC)     // WLIST5
	rs.bitsP0(0x2D, 0, 0x70)   // BLEMATCH_CFG0: WL_MATCH_NONE
	rs.bitsP0(0x2D, 1, 0x0C)   // BLEMATCH_CFG0: BLELEN_MATCH_EQUAL
	// SetWhiteInitVal(ch.whInit):
	rs.bitsP0(0x1A, ch.whInit, 0x7F) // SCR_CFG bits[6:0] = whitening seed

	// ── EnableTxNoAck(1) ─────────────────────────────────────────────────────
	rs.bitsP0(0x08, 0, 0x04) // WMODE_CFG1: NORMAL_M1=0
	rs.bitsP0(0x07, 0, 0x02) // WMODE_CFG0: TX_NOACK_EN=0 (always clear)
	// EnableFifo128bytes(True):
	rs.bitsP0(0x08, 1, 0x20) // WMODE_CFG1: FIFO_128=1
	// SetWaitAckTimeout(2000 µs): 2000 = 0x07D0
	rs.writeP0(0x2B, 0xD0) // RXTIMEOUTL
	rs.writeP0(0x2C, 0x07) // RXTIMEOUTH
	// SetAutoRetrans(0, 0):
	rs.bitsP0(0x29, 0, 0xF0) // TXAUTO_CFG: ARD=0
	rs.bitsP0(0x29, 0, 0x0F) // TXAUTO_CFG: ARC=0

	// ── EnableInterfaceMuxIRQ(IOMUX_EN=0) ────────────────────────────────────
	if useI2C {
		rs.bitsP0(0x03, 0, 0x04) // SYS_CFG: IRQ_MOSI_MUX_EN=0
		rs.bitsP0(0x06, 0, 0x08) // LP_CFG: IRQ_I2C_MUX_EN=0 (IOMUX_EN=0 → no IRQ mux)
	}

	// ── ConfigIT(InterruptMask=0xFF) ─────────────────────────────────────────
	// 0xFF means all IRQs enabled (RFIRQ_CFG = 255-255 = 0). Adjust as needed.
	// ES_Tool default uses InterruptMask=15 → RFIRQ_CFG=0xF0; for TX-only use 0.
	rs.writeP0(0x0B, 0x00) // RFIRQ_CFG: all IRQs unmasked (TX_IRQ=enabled)

	// ── SetXTALFreq(16MHz, EN_AGC=0, DR=1Mbps) ───────────────────────────────
	if xtalMHz == 16 {
		rs.writeP1(0x41, 0xA6) // P1_VCO_PA_CTL: 16 MHz crystal
		// P1[0x3F] and P1[0x40] only for 2Mbps or AGC; skip for 1Mbps non-AGC.
	} else {
		rs.writeP1(0x41, 0xA2) // P1_VCO_PA_CTL: 32 MHz crystal
	}

	// ── SetRxGain(LowGain, EN_AGC=0) ─────────────────────────────────────────
	rs.writeP0(0x61, 0x2E)    // RF_GAIN_WORD3 = 46
	rs.bitsP0(0x4E, 46, 0x3F) // RF_AGC_GAIN_CFG: GAIN_OVRD=46 (preserves GAIN_SEL=1)
}

// ── output generation ─────────────────────────────────────────────────────────

// regWrite is a (addr, value) pair for one page.
type regWrite struct {
	addr, val uint8
	comment   string
}

// knownConstants maps (page, addr) → constant name in registers.go.
var knownConstants = map[[2]uint8]string{
	{0, 0x02}: "STATE_CFG",
	{0, 0x07}: "WMODE_CFG0",
	{0, 0x08}: "WMODE_CFG1",
	{0, 0x09}: "RXPLLEN_CFG",
	{0, 0x0A}: "TXPLLEN_CFG",
	{0, 0x0B}: "RFIRQ_CFG",
	{0, 0x19}: "PKT_EXT_CFG",
	{0, 0x1A}: "WHITEN_CFG",
	{0, 0x1B}: "TXHDR0_CFG",
	{0, 0x1C}: "TXHDR1_CFG",
	{0, 0x1F}: "RXPIPE_CFG",
	{0, 0x29}: "TXAUTO_CFG",
	{0, 0x2A}: "TRXMODE_CFG",
	{0, 0x2B}: "RXTIMEOUTL_CFG",
	{0, 0x2C}: "RXTIMEOUTH_CFG",
	{0, 0x2D}: "BLEMATCH_CFG0",
	{0, 0x30}: "WLIST1_CFG",
	{0, 0x31}: "WLIST2_CFG",
	{0, 0x32}: "WLIST3_CFG",
	{0, 0x33}: "WLIST4_CFG",
	{0, 0x34}: "WLIST5_CFG",
	{0, 0x35}: "BLEMATCHSTART_CFG",
	{0, 0x37}: "RF_OSC_CFG",
	{0, 0x0F}: "PIPE0_RXADDR0",
	{0, 0x20}: "PIPE1_RXADDR0",
	{0, 0x25}: "PIPE2_RXADDR0",
	{0, 0x26}: "PIPE3_RXADDR0",
	{0, 0x27}: "PIPE4_RXADDR0",
	{0, 0x28}: "PIPE5_RXADDR0",
	{0, 0x39}: "RF_CHANNEL_CFG",
	{0, 0x43}: "RF_PA_MODE_CFG",
	{0, 0x44}: "RF_PA_POUT_CFG",
	{0, 0x4E}: "RF_AGC_GAIN_CFG",
	{0, 0x55}: "RF_RSSI_TH1",
	{0, 0x56}: "RF_RSSI_TH2",
	{0, 0x57}: "RF_RSSI_TH3",
	{0, 0x5A}: "RF_RSSI_FIX0",
	{0, 0x5B}: "RF_RSSI_FIX1",
	{0, 0x5C}: "RF_RSSI_FIX2",
	{0, 0x61}: "RF_GAIN_WORD3",
	{0, 0x6F}: "MISC_CFG",
	{1, 0x27}: "P1_RF_TUNE_27",
	{1, 0x32}: "P1_RF_TUNE_32",
	{1, 0x33}: "P1_RF_TUNE_33",
	{1, 0x3A}: "P1_RF_TUNE_3A",
	{1, 0x3C}: "P1_TX_PWR_AMP",
	{1, 0x3E}: "P1_RF_TUNE_3E",
	{1, 0x41}: "P1_VCO_PA_CTL",
	{1, 0x46}: "P1_PA_BIAS",
	{1, 0x48}: "P1_RF_TUNE_48",
	{1, 0x4C}: "P1_RF_TUNE_4C",
}

// knownValueNames maps value → constant name for selected values.
var knownValueNames = map[uint8]string{
	0x42: "0x42", // ADV_NONCONN_IND | TxAdd=1 (no single constant for this combo)
}

func constName(page uint8, addr uint8) string {
	if name, ok := knownConstants[[2]uint8{page, addr}]; ok {
		return name
	}
	return fmt.Sprintf("0x%02X", addr)
}

func printInitTable(title string, page uint8, p0def, p0new [128]uint8) {
	fmt.Printf("\t// ── %s ──\n", title)
	for addr := 0; addr < 128; addr++ {
		dflt := p0def[addr]
		cur := p0new[addr]
		if cur == dflt {
			continue // skip unchanged registers
		}
		name := constName(page, uint8(addr))
		fmt.Printf("\t{%s, 0x%02X},", name, cur)
		// comment: what changed
		fmt.Printf("\t// was 0x%02X\n", dflt)
	}
}

func printGoFile(rs *regState, defaults *regState, ch BLEChannel, xtalMHz, powerDBm int,
	payloadLen uint8, iface string, pkg string) {

	fmt.Printf("// Code generated by pan211x-config. DO NOT EDIT.\n")
	fmt.Printf("// BLE ADV_NONCONN_IND beacon, CH%d, %d MHz crystal, %+d dBm, %s.\n",
		ch.index, xtalMHz, powerDBm, strings.ToUpper(iface))
	fmt.Printf("// Payload: %d bytes (AdvA + AdvData, header auto-inserted).\n", payloadLen)
	fmt.Printf("//\n")
	fmt.Printf("// Apply P1 writes with PAGE_CFG=1 before RF calibration (Step 4 of Init).\n")
	fmt.Printf("// Apply P0 writes with PAGE_CFG=0 after OTP read (Step 5 of Init).\n")
	fmt.Printf("// BLE AdvA goes in the TX FIFO payload, not in the pipe address registers.\n")
	fmt.Println()
	fmt.Printf("package %s\n\n", pkg)

	fmt.Printf("// BLEInitP1 is the Page 1 register init table for the configuration above.\n")
	fmt.Printf("// Replace the p1 table in Init() Step 4 with this slice.\n")
	fmt.Printf("var BLEInitP1 = []struct{ reg, val uint8 }{\n")
	for addr := 0; addr < 128; addr++ {
		dflt := defaults.p1[addr]
		cur := rs.p1[addr]
		if cur == dflt {
			continue
		}
		name := constName(1, uint8(addr))
		fmt.Printf("\t{%s, 0x%02X}, // was 0x%02X\n", name, cur, dflt)
	}
	fmt.Printf("}\n\n")

	fmt.Printf("// BLEInitP0 is the Page 0 register init table for the configuration above.\n")
	fmt.Printf("// Apply all writes after entering Page 0 in Init() Step 5.\n")
	fmt.Printf("// Note: pipe addresses and OwnAddr are set separately via Config.\n")
	fmt.Printf("var BLEInitP0 = []struct{ reg, val uint8 }{\n")
	// Output P0 in a logical order: protocol → timing → channel → RF analog
	order := []uint8{
		0x37,                   // RF_OSC_CFG
		0x07, 0x08,             // WMODE_CFG0/1
		0x09, 0x0A,             // payload lengths
		0x0B,                   // RFIRQ_CFG
		0x19, 0x1A, 0x1B, 0x1C, // PKT config, whitening, headers
		0x1F,                   // RXPIPE_CFG
		// pipe 1 address: skip (set from OwnAddr in Init)
		0x29, 0x2A, 0x2B, 0x2C, // timing/mode
		0x2D, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, // BLE match
		0x39,                   // RF channel
		0x43, 0x44,             // RF analog power
		0x4E,                   // AGC gain
		0x57, 0x5A, 0x5B, 0x5C, 0x61, // RSSI / gain words
	}
	written := map[uint8]bool{}
	for _, addr := range order {
		dflt := defaults.p0[addr]
		cur := rs.p0[addr]
		written[addr] = true
		if cur == dflt {
			continue
		}
		name := constName(0, addr)
		fmt.Printf("\t{%s, 0x%02X}, // was 0x%02X\n", name, cur, dflt)
	}
	// any remaining changed registers not in the explicit order
	for addr := 0; addr < 128; addr++ {
		if written[uint8(addr)] {
			continue
		}
		dflt := defaults.p0[addr]
		cur := rs.p0[addr]
		if cur == dflt {
			continue
		}
		name := constName(0, uint8(addr))
		fmt.Printf("\t{%s, 0x%02X}, // was 0x%02X\n", name, cur, dflt)
	}
	fmt.Printf("}\n")
}

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	channel := flag.Int("channel", 37, "BLE advertising channel (37, 38, or 39)")
	xtal := flag.Int("xtal", 16, "crystal frequency MHz (16 or 32)")
	power := flag.Int("power", 9, "TX power in dBm (9, 0, -5, -10, etc.)")
	payload := flag.Int("payload", 19, "payload length bytes: AdvA(6)+AdvData")
	iface := flag.String("iface", "i2c", "interface mode: i2c or spi")
	pkg := flag.String("pkg", "pan211x", "Go package name for generated file")
	flag.Parse()

	ch, ok := bleChannels[*channel]
	if !ok {
		fmt.Fprintf(os.Stderr, "error: --channel must be 37, 38, or 39 (got %d)\n", *channel)
		os.Exit(1)
	}
	if *xtal != 16 && *xtal != 32 {
		fmt.Fprintf(os.Stderr, "error: --xtal must be 16 or 32 (got %d)\n", *xtal)
		os.Exit(1)
	}
	pwr, ok := findPower(*power)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unsupported --power %d dBm\n", *power)
		fmt.Fprintf(os.Stderr, "supported: 11 9 8 7 6 5 4 3 2 1 0 -1 -2 -5 -7 -8 -10 -11 -12 -14 -16 -19 -23 -25 -28 -33 -37 -40\n")
		os.Exit(1)
	}
	if *payload < 6 || *payload > 37 {
		fmt.Fprintf(os.Stderr, "error: --payload must be 6..37 (AdvA=6 + AdvData 0..31)\n")
		os.Exit(1)
	}
	useI2C := *iface == "i2c"
	if !useI2C && *iface != "spi" {
		fmt.Fprintf(os.Stderr, "error: --iface must be i2c or spi\n")
		os.Exit(1)
	}

	defaults := newRegState()
	rs := newRegState()
	setupConfigBLE(rs, ch, *xtal, pwr, uint8(*payload), useI2C)

	printGoFile(rs, defaults, ch, *xtal, *power, uint8(*payload), *iface, *pkg)
}
