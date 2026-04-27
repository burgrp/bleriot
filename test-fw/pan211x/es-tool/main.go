// pan211x-config: generates PAN211x register initialization tables.
//
// Ports the ES_Tool SetupConfig() logic (modified_genconfig.py) to Go.
// Outputs a Go source fragment with P1 and P0 register init tables for
// the selected chip mode, data rate, TX power, and interface.
//
// Usage:
//
//	go run ./es-tool --mode xn297l --rate 1m --power 9 --xtal 32 --iface spi3
//	go run ./es-tool --mode ble --power 9 --xtal 16 --iface i2c
//
// Chip modes: xn297l (default), fs01, fs32, ble
// For XN297L / FS modes: channel and addresses are static init parameters.
// For BLE: RF_CHANNEL_CFG is included; driver overrides it per ADV channel hop.
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"math/bits"
	"os"
	"strings"
)

// ── constants ──────────────────────────────────────────────────────────────────

const (
	chipXN297 uint8 = 0
	chipFS01  uint8 = 1
	chipFS32  uint8 = 2
	chipBLE   uint8 = 3

	dr1M   uint8 = 0
	dr2M   uint8 = 1
	dr250K uint8 = 2

	ifI2C  uint8 = 1
	ifSPI3 uint8 = 2
	ifSPI4 uint8 = 3

	crcOff uint8 = 0
	crc1   uint8 = 1
	crc2   uint8 = 2
	crc3   uint8 = 3

	wmNormal  uint8 = 0
	wmEnhance uint8 = 1

	s2s8Dis uint8 = 0
	s2s8S2  uint8 = 1
	s2s8S8  uint8 = 2

	endianBig    uint8 = 1
	endianLittle uint8 = 0

	rxLowGain  uint8 = 0
	rxHighGain uint8 = 1

	txDev250K uint8 = 0 // XN297L 1 Mbps TX deviation 250 KHz (default)
	txDev300K uint8 = 1 // XN297L 1 Mbps TX deviation 300 KHz

	bleLenDisable uint8 = 0
	bleLenEqual   uint8 = 1
	bleLenExceed  uint8 = 2
	bleLenBeneath uint8 = 3
)

// ── TX / RX demodulator tables ────────────────────────────────────────────────

// txDemodRows: two register writes driven by a table column index.
var txDemodRows = [2]struct{ pg, addr, mask uint8 }{
	{1, 0x32, 0x1F}, // P1[0x32] bits[4:0]
	{1, 0x33, 0x3F}, // P1[0x33] bits[5:0]
}

// txDemodVals[row][col]: 15 columns, indexed by txDemodColFor().
var txDemodVals = [2][15]uint8{
	{30, 31, 16, 24, 31, 16, 16, 31, 16, 16, 31, 16, 24, 31, 16},
	{25, 63, 28, 27, 50, 28, 25, 26, 25, 28, 28, 28, 27, 50, 28},
}

// txDemodColFor returns the column for (chip, rate, txDev).
// txDev only matters for chipXN297 + dr1M.
func txDemodColFor(chip, rate, txDev uint8) int {
	switch {
	case chip == chipXN297 && rate == dr1M:
		if txDev == txDev250K {
			return 3
		}
		return 0 // txDev300K
	case chip == chipXN297 && rate == dr2M:
		return 4
	case chip == chipXN297 && rate == dr250K:
		return 5
	case chip == chipFS01 && rate == dr1M:
		return 6
	case chip == chipFS01 && rate == dr2M:
		return 7
	case chip == chipFS01 && rate == dr250K:
		return 8
	case chip == chipFS32 && rate == dr1M:
		return 9
	case chip == chipFS32 && rate == dr2M:
		return 10
	case chip == chipFS32 && rate == dr250K:
		return 11
	case chip == chipBLE && rate == dr1M:
		return 12
	case chip == chipBLE && rate == dr2M:
		return 13
	default: // BLE 250K
		return 14
	}
}

// rxDemodTable: 16 rows × 6 columns.
// SetupConfig always calls SetDataRate with s2s8=0, so only columns 0–4 are used.
type rxDemodRow struct {
	pg, addr, mask uint8
	vals           [6]uint8
}

var rxDemodTable = [16]rxDemodRow{
	{0, 0x38, 0x1F, [6]uint8{16, 16, 16, 11, 11, 19}},
	{0, 0x38, 0x60, [6]uint8{0, 0, 2, 0, 2, 0}},
	{0, 0x37, 0x7F, [6]uint8{96, 96, 96, 107, 107, 90}},
	{0, 0x36, 0x80, [6]uint8{0, 0, 1, 0, 1, 0}},
	{0, 0x36, 0x40, [6]uint8{1, 1, 0, 1, 0, 1}},
	{0, 0x36, 0x0F, [6]uint8{5, 5, 0, 4, 0, 6}},
	{1, 0x07, 0xF0, [6]uint8{7, 7, 7, 5, 7, 8}},
	{1, 0x07, 0x0F, [6]uint8{5, 5, 5, 4, 5, 6}},
	{1, 0x0D, 0x3F, [6]uint8{9, 9, 9, 6, 9, 11}},
	{1, 0x0F, 0x1F, [6]uint8{15, 15, 15, 10, 15, 18}},
	{1, 0x0E, 0x80, [6]uint8{0, 1, 1, 0, 0, 0}},
	{1, 0x0E, 0x40, [6]uint8{1, 0, 0, 1, 1, 1}},
	{1, 0x15, 0x40, [6]uint8{1, 0, 0, 1, 1, 1}},
	{1, 0x5C, 0x80, [6]uint8{1, 0, 0, 1, 1, 1}},
	{1, 0x5D, 0x40, [6]uint8{0, 1, 1, 0, 0, 0}},
	{1, 0x0A, 0x80, [6]uint8{1, 0, 0, 1, 0, 1}},
}

// rxDemodColFor returns column index for (chip, rate) with s2s8 always DIS.
func rxDemodColFor(chip, rate uint8) int {
	if chip == chipFS01 || chip == chipFS32 {
		if rate == dr250K {
			return 4
		}
		return 3 // FS01/FS32 1M and 2M both use col 3
	}
	if rate == dr250K {
		return 4
	}
	return 0 // XN297L and BLE, 1M and 2M
}

// ── TX power table ────────────────────────────────────────────────────────────

// TxPower holds the six register values from sop8_power_table.
// dBm 99 is the special "0 dBm low-power" variant.
type TxPower struct {
	dBm      int
	poutRes  uint8 // P1[0x3C] bits[2:0]
	modePA   uint8 // P0[0x43] bits[5:4]
	poutCrnt uint8 // P0[0x44] bits[7:4]
	ldoSel   uint8 // P0[0x44] bits[3:0]
	mtchC2TX uint8 // P1[0x46] bit0
	mtchC1TX uint8 // P1[0x46] bits[3:2]
}

var sop8PowerTable = []TxPower{
	{11, 7, 3, 15, 12, 0, 0},
	{9, 7, 3, 8, 12, 0, 0},
	{8, 7, 3, 8, 6, 0, 0},
	{7, 7, 3, 8, 3, 0, 0},
	{6, 7, 3, 8, 4, 0, 1},
	{5, 7, 3, 8, 2, 0, 1},
	{4, 7, 3, 8, 0, 0, 1},
	{3, 7, 3, 8, 0, 0, 2},
	{2, 3, 3, 8, 2, 0, 3},
	{1, 3, 3, 8, 0, 0, 3},
	{0, 3, 3, 8, 4, 1, 3},
	{99, 7, 1, 8, 15, 0, 0}, // 0 dBm low-power
	{-1, 4, 3, 8, 0, 1, 3},
	{-2, 7, 1, 15, 15, 0, 1},
	{-5, 7, 1, 15, 15, 1, 3},
	{-7, 3, 1, 8, 8, 1, 3},
	{-8, 3, 1, 8, 4, 1, 1},
	{-10, 3, 1, 8, 0, 1, 0},
	{-11, 3, 1, 6, 0, 1, 0},
	{-12, 3, 1, 5, 0, 1, 0},
	{-14, 3, 1, 4, 0, 1, 0},
	{-16, 3, 1, 3, 0, 1, 0},
	{-19, 3, 1, 2, 0, 1, 0},
	{-23, 3, 1, 1, 0, 1, 0},
	{-25, 2, 1, 1, 0, 1, 0},
	{-28, 1, 1, 1, 8, 1, 0},
	{-33, 3, 1, 0, 0, 1, 0},
	{-37, 0, 1, 0, 0, 0, 0},
	{-40, 0, 1, 0, 0, 1, 0},
}

func findPower(dBm int) (TxPower, bool) {
	for _, p := range sop8PowerTable {
		if p.dBm == dBm {
			return p, true
		}
	}
	return TxPower{}, false
}

// ldoRffe returns P1[0x48] bits[3:0] for the given power level.
func ldoRffe(dBm int) uint8 {
	switch dBm {
	case 11:
		return 15
	case 99: // 0 dBm low-power
		return 12
	default:
		return 8
	}
}

// ldoPA returns P1[0x3C] bit3 for the given power level.
func ldoPA(dBm int) uint8 {
	if dBm == 11 {
		return 1
	}
	return 0
}

// codeOffset returns P1[0x27] (TX code offset) for chip/rate/power.
func codeOffset(chip, rate uint8, dBm int) uint8 {
	switch rate {
	case dr2M:
		return 0xCA
	case dr250K:
		return 0x0A
	default: // 1M
		return 0xAA
	}
}

// ── Config ────────────────────────────────────────────────────────────────────

// Config mirrors PAN211MPCONFIG — all parameters for SetupConfig.
type Config struct {
	// Universal
	ChipMode   uint8
	DataRate   uint8
	TxPowerDBm int    // dBm; 99 = 0 dBm low-power variant
	XtalMHz    int    // 16 or 32
	Iface      uint8  // ifI2C | ifSPI3 | ifSPI4
	TxMode     uint8  // 0 = single, 1 = continuous
	RxMode     uint8  // 0 = single, 1 = single+timeout, 2 = continuous
	RxGain     uint8  // rxLowGain | rxHighGain
	EN_AGC     bool
	IOMUX_EN   bool
	IntMask    uint8  // interrupt mask passed to ConfigIT (0 = all enabled)
	EnRxLimit  bool   // RxLengthLimit
	TxDevSel   uint8  // txDev250K | txDev300K (XN297L 1 Mbps only)

	// XN297L / FS01 / FS32
	Endian      uint8  // endianBig | endianLittle (FS32 user-selectable; others fixed)
	CrcSkipAddr bool   // FS32 only
	AddrWidth   uint8  // 2–5 bytes
	Crc         uint8  // crcOff | crc1 | crc2 | crc3
	EnWhite     bool
	EnDPL       bool
	WorkMode    uint8 // wmNormal | wmEnhance
	EnTxNoAck   bool
	RxTimeoutUs uint16
	TRxDelayUs  uint16
	AutoDelayUs uint16
	AutoMaxCnt  uint8

	// BLE
	S2S8Mode           uint8
	BLEHeadNum         int
	BLEHead0           uint8
	BLEHead1           uint8
	WhiteInit          uint8
	WhiteListMatchMode uint8
	WhiteListOffset    uint8
	WhiteList          [6]byte
	WhiteListLen       uint8
	LenFilterMode      uint8

	Pkg string
}

// ── register simulation ───────────────────────────────────────────────────────

type regState struct {
	p0 [128]uint8
	p1 [128]uint8
}

func newRegState() *regState {
	rs := &regState{}
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

func (rs *regState) writeP0(addr, val uint8)  { rs.p0[addr] = val }
func (rs *regState) writeP1(addr, val uint8)  { rs.p1[addr] = val }

func (rs *regState) bitsP0(addr, bitval, mask uint8) {
	shift := trailingZeros8(mask)
	rs.p0[addr] = (rs.p0[addr] &^ mask) | ((bitval<<shift)&mask)
}
func (rs *regState) bitsP1(addr, bitval, mask uint8) {
	shift := trailingZeros8(mask)
	rs.p1[addr] = (rs.p1[addr] &^ mask) | ((bitval<<shift)&mask)
}

// ── setup helpers (ported from modified_genconfig.py) ────────────────────────

func writeRecommendedRegs(rs *regState, enAGC bool) {
	rs.writeP1(0x27, 0xCA) // CodeOffset initial (overwritten by setTxPower)
	if enAGC {
		rs.writeP1(0x37, 0x15)
		rs.writeP1(0x3A, 0x14)
	}
	rs.writeP1(0x3E, 0xF1)
	rs.writeP0(0x09, 0x03) // overwritten by setRxPayloadLen
	rs.writeP0(0x0A, 0x03) // overwritten by setTxPayloadLen
	rs.writeP0(0x39, 0x55) // overwritten by setChannel
	if enAGC {
		rs.writeP0(0x43, 0x3A)
		rs.writeP0(0x55, 0xDD)
		rs.writeP0(0x56, 0xC9)
		rs.writeP0(0x57, 0xB7)
		rs.writeP0(0x5A, 0x10)
		rs.writeP0(0x5B, 0xFD)
		rs.writeP0(0x5C, 0xE9)
		rs.writeP0(0x5D, 0xDC)
		rs.writeP0(0x5E, 0x02)
		rs.writeP0(0x5F, 0x06)
		rs.writeP0(0x60, 0x0E)
		rs.writeP0(0x61, 0x2E)
		rs.writeP0(0x66, 0x34)
		rs.writeP0(0x68, 0x0D)
		rs.writeP0(0x6E, 0x20)
	} else {
		rs.writeP0(0x4E, 0x7E)
		rs.writeP0(0x57, 0xDD)
		rs.writeP0(0x5A, 0xCD)
		rs.writeP0(0x5B, 0xCD)
		rs.writeP0(0x5C, 0xCD)
		rs.writeP0(0x61, 0x2E)
	}
}

func setPredefinedRegs(rs *regState, iface uint8, xtalMHz int) {
	rs.writeP0(0x02, 0x74)
	if xtalMHz == 16 {
		rs.writeP0(0x37, 0xE0)
	}
	switch iface {
	case ifSPI3:
		rs.writeP0(0x04, 0x83) // SPI_CFG: 3-wire
		rs.writeP0(0x03, 0x02) // SYS_CFG: 3-wire
	case ifSPI4:
		rs.writeP0(0x04, 0x03) // SPI_CFG: 4-wire
		rs.writeP0(0x03, 0x03) // SYS_CFG: 4-wire
	}
	// I2C: no SPI_CFG writes
}

func setChipMode(rs *regState, chip, endian uint8, crcSkipAddr bool) {
	switch chip {
	case chipXN297:
		rs.bitsP0(0x07, 0, 0x20) // CHIP_MODE=0
		rs.bitsP0(0x6F, 0, 0x10) // PID_LOW_SEL=0 (big-endian)
		rs.bitsP0(0x07, 1, 0x01) // ENDIAN=1 (big)
		rs.bitsP0(0x07, 0, 0x04) // ACCADDR_CRC_DIS=0
		rs.bitsP0(0x1A, 0, 0x80) // ACCADDR_SCR_DIS=0
		rs.bitsP0(0x1A, 0x7F, 0x7F) // WhiteInit=127
	case chipFS01:
		rs.bitsP0(0x07, 1, 0x20) // CHIP_MODE=1
		rs.bitsP0(0x07, 0, 0x10) // NORDIC_ENHANCE=0
		rs.bitsP0(0x6F, 0, 0x10) // PID_LOW_SEL=0 (big-endian)
		rs.bitsP0(0x07, 1, 0x01) // ENDIAN=1 (big)
		rs.bitsP0(0x07, 0, 0x04) // ACCADDR_CRC_DIS=0
		rs.bitsP0(0x1A, 0, 0x80) // ACCADDR_SCR_DIS=0
		rs.bitsP0(0x1A, 0x7F, 0x7F) // WhiteInit=127
	case chipFS32:
		rs.bitsP0(0x07, 1, 0x20) // CHIP_MODE=1
		rs.bitsP0(0x07, 1, 0x10) // NORDIC_ENHANCE=1
		rs.bitsP0(0x6F, 1, 0x10) // PID_LOW_SEL=1
		if endian == endianLittle {
			rs.bitsP0(0x07, 0, 0x01) // ENDIAN=0
		} else {
			rs.bitsP0(0x07, 1, 0x01) // ENDIAN=1
		}
		if crcSkipAddr {
			rs.bitsP0(0x07, 1, 0x04)
		} else {
			rs.bitsP0(0x07, 0, 0x04)
		}
		rs.bitsP0(0x1A, 1, 0x80)     // ACCADDR_SCR_DIS=1
		rs.bitsP0(0x1A, 0x7F, 0x7F)  // WhiteInit=127
	case chipBLE:
		rs.bitsP0(0x07, 1, 0x20) // CHIP_MODE=1
		rs.bitsP0(0x07, 1, 0x10) // NORDIC_ENHANCE=1
		rs.bitsP0(0x6F, 1, 0x10) // PID_LOW_SEL=1 (little-endian marker)
		rs.bitsP0(0x07, 0, 0x01) // ENDIAN=0 (little)
		rs.bitsP0(0x07, 1, 0x04) // ACCADDR_CRC_DIS=1
		rs.bitsP0(0x1A, 1, 0x80) // ACCADDR_SCR_DIS=1
	}
	rs.bitsP0(0x08, 1, 0x80) // WMODE_CFG1: RX_GOON=1
}

func setChannel(rs *regState, ch uint8) {
	rs.writeP0(0x39, ch)
}

func setDataRate(rs *regState, chip, rate, txDev uint8) {
	var bwMode uint8
	switch rate {
	case dr1M:
		bwMode = 0
	case dr2M:
		bwMode = 1
	case dr250K:
		bwMode = 3
	}
	rs.bitsP0(0x36, bwMode, 0x30) // BW_MODE

	// DRModConfig
	switch rate {
	case dr1M:
		if chip == chipFS01 || chip == chipFS32 {
			rs.bitsP1(0x49, 0, 0x80) // TX_DAC_GC=0
		} else {
			rs.bitsP1(0x49, 1, 0x80) // TX_DAC_GC=1
		}
		rs.bitsP0(0x43, 0, 0x04) // FSYNVCO_TXCTK=0
		rs.bitsP0(0x43, 2, 0x03) // RXFLTR_IF=2
		rs.bitsP1(0x3A, 0, 0xC0) // RXFLTRTUNE_OFST=0
		rs.bitsP1(0x49, 4, 0x70) // TX_DAC_ISEL=4
		rs.bitsP1(0x4C, 0, 0x20) // TX_DAC_BW=0
	case dr250K:
		rs.bitsP0(0x43, 0, 0x04) // FSYNVCO_TXCTK=0
		rs.bitsP0(0x43, 3, 0x03) // RXFLTR_IF=3
		rs.bitsP1(0x3A, 1, 0xC0) // RXFLTRTUNE_OFST=1
		rs.bitsP1(0x49, 0, 0x80) // TX_DAC_GC=0
		rs.bitsP1(0x49, 4, 0x70) // TX_DAC_ISEL=4
		rs.bitsP1(0x4C, 0, 0x20) // TX_DAC_BW=0
	case dr2M:
		if chip == chipFS01 || chip == chipFS32 {
			rs.bitsP0(0x43, 0, 0x04) // FSYNVCO_TXCTK=0
			rs.bitsP1(0x49, 6, 0x70) // TX_DAC_ISEL=6
			rs.bitsP1(0x4C, 0, 0x20) // TX_DAC_BW=0
		} else {
			rs.bitsP0(0x43, 1, 0x04) // FSYNVCO_TXCTK=1
			rs.bitsP1(0x49, 4, 0x70) // TX_DAC_ISEL=4
			rs.bitsP1(0x4C, 1, 0x20) // TX_DAC_BW=1
		}
		rs.bitsP0(0x43, 2, 0x03) // RXFLTR_IF=2
		rs.bitsP1(0x3A, 0, 0xC0) // RXFLTRTUNE_OFST=0
		rs.bitsP1(0x49, 1, 0x80) // TX_DAC_GC=1
	}

	// TxDemodConfig
	col := txDemodColFor(chip, rate, txDev)
	for i := 0; i < 2; i++ {
		r := txDemodRows[i]
		if r.pg == 0 {
			rs.bitsP0(r.addr, txDemodVals[i][col], r.mask)
		} else {
			rs.bitsP1(r.addr, txDemodVals[i][col], r.mask)
		}
	}

	// RxDemodConfig (always with s2s8=DIS)
	col = rxDemodColFor(chip, rate)
	for _, row := range rxDemodTable {
		if row.pg == 0 {
			rs.bitsP0(row.addr, row.vals[col], row.mask)
		} else {
			rs.bitsP1(row.addr, row.vals[col], row.mask)
		}
	}
}

func setTxPower(rs *regState, chip, rate uint8, pwr TxPower) {
	rs.bitsP1(0x3C, pwr.poutRes, 0x07)
	rs.bitsP0(0x43, pwr.modePA, 0x30)
	rs.bitsP0(0x44, pwr.poutCrnt, 0xF0)
	rs.bitsP0(0x44, pwr.ldoSel, 0x0F)
	rs.bitsP1(0x46, pwr.mtchC2TX, 0x01)
	rs.bitsP1(0x46, pwr.mtchC1TX, 0x0C)
	rs.writeP1(0x27, codeOffset(chip, rate, pwr.dBm))
	rs.bitsP1(0x48, ldoRffe(pwr.dBm), 0x0F)
	rs.bitsP1(0x3C, ldoPA(pwr.dBm), 0x08)
}

func setTxMode(rs *regState, mode uint8) {
	rs.bitsP0(0x2A, mode, 0x80)
}

func setRxMode(rs *regState, mode uint8) {
	rs.bitsP0(0x2A, mode, 0x60)
}

func setAddrWidth(rs *regState, width uint8) {
	rs.bitsP0(0x08, width-2, 0x03)
}

func setTxAddr(rs *regState, addr []byte) {
	for i, b := range addr {
		rs.writeP0(uint8(0x14+i), b)
	}
}

func setRxAddr(rs *regState, pipe uint8, addr []byte, width uint8) {
	base := []uint8{0x0F, 0x20, 0x25, 0x26, 0x27, 0x28}
	if pipe <= 1 {
		for i := uint8(0); i < width; i++ {
			rs.writeP0(base[pipe]+i, addr[i])
		}
	} else {
		rs.writeP0(base[pipe], addr[0])
	}
}

func enableRxPipe(rs *regState, pipe uint8) {
	rs.bitsP0(0x1F, 1, 1<<pipe)
}

func disableRxPipe(rs *regState, pipe uint8) {
	rs.bitsP0(0x1F, 0, 1<<pipe)
}

func enableDPL(rs *regState, en bool) {
	v := uint8(0)
	if en {
		v = 1
	}
	rs.bitsP0(0x08, v, 0x10)
}

func enableWhiten(rs *regState, en bool) {
	v := uint8(0)
	if en {
		v = 1
	}
	rs.bitsP0(0x07, v, 0x08)
}

func setCrc(rs *regState, scheme uint8) {
	rs.bitsP0(0x07, scheme, 0xC0)
}

func setWorkMode(rs *regState, mode uint8) {
	if mode == wmNormal {
		rs.bitsP0(0x08, 0, 0x0C) // ENHANCE=0, NORMAL_M1=0
	} else {
		rs.bitsP0(0x08, 1, 0x08) // ENHANCE=1
		rs.bitsP0(0x08, 0, 0x04) // NORMAL_M1=0
	}
}

func enableTxNoAck(rs *regState, enable bool) {
	isEnhance := rs.p0[0x08]&0x08 != 0
	if isEnhance {
		if enable {
			rs.bitsP0(0x07, 1, 0x02)
		} else {
			rs.bitsP0(0x07, 0, 0x02)
		}
		rs.bitsP0(0x08, 0, 0x04)
	} else {
		if enable {
			rs.bitsP0(0x08, 0, 0x04)
		} else {
			rs.bitsP0(0x08, 1, 0x04)
		}
	}
	rs.bitsP0(0x07, 0, 0x02) // always clear TX_NOACK_EN
}

func enableFifo128(rs *regState, en bool) {
	v := uint8(0)
	if en {
		v = 1
	}
	rs.bitsP0(0x08, v, 0x20)
}

func setWaitAckTimeout(rs *regState, us uint16) {
	rs.writeP0(0x2B, uint8(us&0xFF))
	rs.writeP0(0x2C, uint8(us>>8))
}

func setTRxTransTime(rs *regState, us uint16) {
	rs.writeP0(0x0D, uint8(us&0xFF))
	rs.writeP0(0x0E, uint8(us>>8))
}

func setAutoRetrans(rs *regState, delayUs uint16, maxCnt uint8) {
	if delayUs < 250 {
		delayUs = 250
	}
	ard := uint8(delayUs/250 - 1)
	rs.bitsP0(0x29, ard, 0xF0)
	rs.bitsP0(0x29, maxCnt, 0x0F)
}

func enableInterfaceMuxIRQ(rs *regState, iomuxEn bool, iface uint8) {
	if !iomuxEn {
		return
	}
	switch iface {
	case ifSPI3:
		rs.bitsP0(0x03, 1, 0x04) // IRQ_MOSI_MUX_EN=1
		rs.bitsP0(0x06, 0, 0x08) // IRQ_I2C_MUX_EN=0
	case ifI2C:
		rs.bitsP0(0x03, 0, 0x04)
		rs.bitsP0(0x06, 1, 0x08) // IRQ_I2C_MUX_EN=1
	default:
		rs.bitsP0(0x03, 0, 0x04)
		rs.bitsP0(0x06, 0, 0x08)
	}
}

func configIT(rs *regState, mask uint8) {
	rs.writeP0(0x0B, 255-mask)
}

func setXTALFreq(rs *regState, xtalMHz int, enAGC bool, rate uint8) {
	if xtalMHz == 16 {
		rs.writeP1(0x41, 0xA6)
		if !enAGC && rate == dr2M {
			rs.writeP1(0x3F, 0xD2)
			rs.writeP1(0x40, 0x20)
		} else if enAGC {
			rs.writeP1(0x3F, 0xD2)
			rs.writeP1(0x40, 0x20)
		}
	} else {
		rs.writeP1(0x41, 0xA2)
	}
}

func setRxGain(rs *regState, gain uint8, enAGC bool) {
	if gain == rxLowGain {
		rs.writeP0(0x61, 0x2E)
		if enAGC {
			rs.writeP0(0x5D, 0xDC)
		} else {
			rs.bitsP0(0x4E, 46, 0x3F)
		}
	} else {
		rs.writeP0(0x61, 0x3E)
		if enAGC {
			rs.writeP0(0x5D, 0xD4)
		} else {
			rs.bitsP0(0x4E, 62, 0x3F)
		}
	}
}

func setS2S8Mode(rs *regState, mode uint8) {
	switch mode {
	case s2s8Dis:
		rs.bitsP0(0x2A, 1, 0x01)  // PRE_SYNC_EN=1
		rs.bitsP0(0x19, 0, 0x03)  // PRI_CI_MODE=0
		rs.bitsP0(0x19, 0, 0x08)  // PRI_TX_FEC=0
		rs.bitsP0(0x19, 0, 0x04)  // PRI_RX_FEC=0
		rs.bitsP1(0x0B, 0, 0x20)  // DEMOD_BLE_LONG_RANGE=0
	case s2s8S2:
		rs.bitsP0(0x2A, 0, 0x01)
		rs.bitsP0(0x19, 1, 0x03)
		rs.bitsP0(0x19, 1, 0x08)
		rs.bitsP0(0x19, 1, 0x04)
		rs.bitsP1(0x0B, 1, 0x20)
	case s2s8S8:
		rs.bitsP0(0x2A, 0, 0x01)
		rs.bitsP0(0x19, 0, 0x03)
		rs.bitsP0(0x19, 1, 0x08)
		rs.bitsP0(0x19, 1, 0x04)
		rs.bitsP1(0x0B, 1, 0x20)
	}
}

func setNordicPktHeader(rs *regState, headerEn bool, headerLen uint8) {
	v := uint8(0)
	if headerEn {
		v = 1
	}
	rs.bitsP0(0x19, v, 0x40)
	rs.bitsP0(0x19, headerLen, 0x30)
}

func writeNordicPktHeader(rs *regState, head0, head1 uint8) {
	rs.writeP0(0x1B, head0)
	rs.writeP0(0x1C, head1)
}

func setBleWhitelist(rs *regState, start uint8, buf []byte) {
	rs.bitsP0(0x35, start, 0x3F)
	startReg := uint8(0x35) - uint8(len(buf))
	for i, b := range buf {
		rs.writeP0(startReg+uint8(i), b)
	}
}

func setBleLenFilter(rs *regState, mode uint8) {
	rs.bitsP0(0x2D, mode, 0x0C)
}

func setBleWLMatchMode(rs *regState, mode uint8) {
	rs.bitsP0(0x2D, mode, 0x70)
}

// ── setupConfig ───────────────────────────────────────────────────────────────

func setupConfig(rs *regState, cfg Config) {
	writeRecommendedRegs(rs, cfg.EN_AGC)
	setPredefinedRegs(rs, cfg.Iface, cfg.XtalMHz)
	rs.bitsP1(0x4C, 0, 0x10) // FSYNXO_STARTUP_FAST=0

	setChipMode(rs, cfg.ChipMode, cfg.Endian, cfg.CrcSkipAddr)
	setDataRate(rs, cfg.ChipMode, cfg.DataRate, cfg.TxDevSel)
	if cfg.EnRxLimit {
		rs.bitsP0(0x19, 1, 0x80)
	} else {
		rs.bitsP0(0x19, 0, 0x80)
	}

	pwr, _ := findPower(cfg.TxPowerDBm)
	setTxPower(rs, cfg.ChipMode, cfg.DataRate, pwr)
	setTxMode(rs, cfg.TxMode)
	setRxMode(rs, cfg.RxMode)

	if cfg.ChipMode != chipBLE {
		enableDPL(rs, cfg.EnDPL)
		enableWhiten(rs, cfg.EnWhite)
		setCrc(rs, cfg.Crc)
		setWorkMode(rs, cfg.WorkMode)
		setAddrWidth(rs, cfg.AddrWidth)
		txAddr := [5]byte{0xCC, 0xCC, 0xCC, 0xCC, 0xCC}
		setTxAddr(rs, txAddr[:cfg.AddrWidth])
		for i := uint8(0); i < 6; i++ {
			enableRxPipe(rs, i)
			addr := [5]byte{0xCC, 0xCC, 0xCC, 0xCC, 0xCC}
			addr[0] += i
			setRxAddr(rs, i, addr[:cfg.AddrWidth], cfg.AddrWidth)
		}
	} else {
		enableDPL(rs, true)
		enableWhiten(rs, true)
		setCrc(rs, crc3)
		setWorkMode(rs, wmNormal)
		setAddrWidth(rs, 4)
		setTxAddr(rs, []byte{0xCC, 0xCC, 0xCC, 0xCC})
		bleRxAddrs := [6]byte{0xC0, 0xC1, 0xC2, 0xC3, 0xC4, 0xC5}
		for i := uint8(0); i < 6; i++ {
			enableRxPipe(rs, i)
			if i <= 1 {
				setRxAddr(rs, i, []byte{bleRxAddrs[i], 0xCC, 0xCC, 0xCC}, 4)
			} else {
				setRxAddr(rs, i, []byte{bleRxAddrs[i]}, 1)
			}
		}
		setS2S8Mode(rs, cfg.S2S8Mode)
		writeNordicPktHeader(rs, cfg.BLEHead0, cfg.BLEHead1)
		if cfg.BLEHeadNum == 0 {
			setNordicPktHeader(rs, false, 0)
		} else {
			setNordicPktHeader(rs, true, uint8(cfg.BLEHeadNum))
		}
		setBleWLMatchMode(rs, cfg.WhiteListMatchMode)
		wl := cfg.WhiteList[:cfg.WhiteListLen]
		if len(wl) == 0 {
			wl = []byte{0xCC, 0xCC, 0xCC, 0xCC, 0xCC}
		}
		setBleWhitelist(rs, cfg.WhiteListOffset, wl)
		setBleLenFilter(rs, cfg.LenFilterMode)
		rs.bitsP0(0x1A, cfg.WhiteInit, 0x7F)
	}

	enableTxNoAck(rs, cfg.EnTxNoAck)
	if cfg.EnTxNoAck {
		enableFifo128(rs, true)
		setWaitAckTimeout(rs, cfg.RxTimeoutUs)
		setAutoRetrans(rs, 0, 0)
	} else {
		enableFifo128(rs, false)
		setTRxTransTime(rs, cfg.TRxDelayUs)
		setWaitAckTimeout(rs, cfg.RxTimeoutUs)
		setAutoRetrans(rs, cfg.AutoDelayUs, cfg.AutoMaxCnt)
	}

	enableInterfaceMuxIRQ(rs, cfg.IOMUX_EN, cfg.Iface)
	configIT(rs, cfg.IntMask)
	setXTALFreq(rs, cfg.XtalMHz, cfg.EN_AGC, cfg.DataRate)
	setRxGain(rs, cfg.RxGain, cfg.EN_AGC)
}

// ── output generation ─────────────────────────────────────────────────────────

var knownConstants = map[[2]uint8]string{
	{0, 0x02}: "STATE_CFG",
	{0, 0x03}: "SYS_CFG",
	{0, 0x04}: "SPI_CFG",
	{0, 0x06}: "LP_CFG",
	{0, 0x07}: "WMODE_CFG0",
	{0, 0x08}: "WMODE_CFG1",
	{0, 0x09}: "RXPLLEN_CFG",
	{0, 0x0A}: "TXPLLEN_CFG",
	{0, 0x0B}: "RFIRQ_CFG",
	{0, 0x0D}: "TRXTWTL_CFG",
	{0, 0x0E}: "TRXTWTH_CFG",
	{0, 0x0F}: "PIPE0_RXADDR0",
	{0, 0x14}: "TXADDR0_CFG",
	{0, 0x19}: "PKT_EXT_CFG",
	{0, 0x1A}: "WHITEN_CFG",
	{0, 0x1B}: "TXHDR0_CFG",
	{0, 0x1C}: "TXHDR1_CFG",
	{0, 0x1F}: "RXPIPE_CFG",
	{0, 0x20}: "PIPE1_RXADDR0",
	{0, 0x25}: "PIPE2_RXADDR0",
	{0, 0x26}: "PIPE3_RXADDR0",
	{0, 0x27}: "PIPE4_RXADDR0",
	{0, 0x28}: "PIPE5_RXADDR0",
	{0, 0x29}: "TXAUTO_CFG",
	{0, 0x2A}: "TRXMODE_CFG",
	{0, 0x2B}: "RXTIMEOUTL_CFG",
	{0, 0x2C}: "RXTIMEOUTH_CFG",
	{0, 0x2D}: "BLEMATCH_CFG0",
	{0, 0x2E}: "BLEMATCH_CFG1",
	{0, 0x2F}: "WLIST0_CFG",
	{0, 0x30}: "WLIST1_CFG",
	{0, 0x31}: "WLIST2_CFG",
	{0, 0x32}: "WLIST3_CFG",
	{0, 0x33}: "WLIST4_CFG",
	{0, 0x34}: "WLIST5_CFG",
	{0, 0x35}: "BLEMATCHSTART_CFG",
	{0, 0x37}: "RF_OSC_CFG",
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
	{1, 0x49}: "P1_RF_TUNE_49",
	{1, 0x4C}: "P1_RF_TUNE_4C",
}

func constName(page, addr uint8) string {
	if name, ok := knownConstants[[2]uint8{page, addr}]; ok {
		return name
	}
	return fmt.Sprintf("0x%02X", addr)
}

func printGoFile(rs *regState, defaults *regState, cfg Config) {
	modeNames := map[uint8]string{chipXN297: "XN297L", chipFS01: "FS01", chipFS32: "FS32", chipBLE: "BLE"}
	rateNames := map[uint8]string{dr1M: "1 Mbps", dr2M: "2 Mbps", dr250K: "250 Kbps"}
	ifaceNames := map[uint8]string{ifI2C: "I2C", ifSPI3: "SPI-3", ifSPI4: "SPI-4"}

	fmt.Printf("// Code generated by pan211x-config. DO NOT EDIT.\n")
	fmt.Printf("// %s %s, %+d dBm, %d MHz xtal, %s.\n",
		modeNames[cfg.ChipMode], rateNames[cfg.DataRate],
		cfg.TxPowerDBm, cfg.XtalMHz, ifaceNames[cfg.Iface])
	fmt.Printf("//\n")
	fmt.Printf("// Apply P1 writes with PAGE_CFG=1 before RF calibration (Step 4 of Init).\n")
	fmt.Printf("// Apply P0 writes with PAGE_CFG=0 after OTP read (Step 5 of Init).\n")
	if cfg.ChipMode == chipBLE {
		fmt.Printf("// BLE: RF_CHANNEL_CFG is overridden at runtime per ADV channel hop.\n")
		fmt.Printf("// BLE: AdvA goes in the TX FIFO payload, not in the pipe address registers.\n")
	}
	fmt.Println()
	fmt.Printf("package %s\n\n", cfg.Pkg)

	fmt.Printf("// InitP1 is the Page 1 register init table.\n")
	fmt.Printf("var InitP1 = []struct{ reg, val uint8 }{\n")
	for addr := 0; addr < 128; addr++ {
		dflt := defaults.p1[addr]
		cur := rs.p1[addr]
		if cur == dflt {
			continue
		}
		fmt.Printf("\t{%s, 0x%02X}, // was 0x%02X\n", constName(1, uint8(addr)), cur, dflt)
	}
	fmt.Printf("}\n\n")

	fmt.Printf("// InitP0 is the Page 0 register init table.\n")
	fmt.Printf("var InitP0 = []struct{ reg, val uint8 }{\n")

	// Ordered output: protocol config first, then RF analog
	order := []uint8{
		0x02,                               // STATE_CFG
		0x03, 0x04,                         // SYS_CFG, SPI_CFG (SPI modes)
		0x07, 0x08,                         // WMODE_CFG0/1
		0x09, 0x0A,                         // payload lengths
		0x0B,                               // RFIRQ_CFG
		0x0D, 0x0E,                         // TRxTwT (Enhance mode)
		0x0F, 0x10, 0x11, 0x12, 0x13,      // Pipe 0 RX addr
		0x14, 0x15, 0x16, 0x17, 0x18,      // TX addr
		0x19, 0x1A, 0x1B, 0x1C,            // PKT config, whitening, headers
		0x1F,                               // RXPIPE_CFG
		0x20, 0x21, 0x22, 0x23, 0x24,      // Pipe 1 RX addr
		0x25, 0x26, 0x27, 0x28,            // Pipes 2-5 RX addr
		0x29, 0x2A, 0x2B, 0x2C,            // timing / mode
		0x2D, 0x2E, 0x2F, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, // BLE match
		0x37, 0x39,                         // RF_OSC_CFG, RF_CHANNEL_CFG
		0x43, 0x44,                         // RF analog power
		0x4E,                               // AGC gain
		0x55, 0x56, 0x57,                   // RSSI thresholds
		0x5A, 0x5B, 0x5C, 0x61,            // RSSI fix / gain
	}
	written := map[uint8]bool{}
	for _, addr := range order {
		written[addr] = true
		dflt := defaults.p0[addr]
		cur := rs.p0[addr]
		if cur == dflt {
			continue
		}
		fmt.Printf("\t{%s, 0x%02X}, // was 0x%02X\n", constName(0, addr), cur, dflt)
	}
	// remaining changed registers not in the explicit order
	for addr := 0; addr < 128; addr++ {
		if written[uint8(addr)] {
			continue
		}
		dflt := defaults.p0[addr]
		cur := rs.p0[addr]
		if cur == dflt {
			continue
		}
		fmt.Printf("\t{%s, 0x%02X}, // was 0x%02X\n", constName(0, uint8(addr)), cur, dflt)
	}
	fmt.Printf("}\n")
}

// ── help ─────────────────────────────────────────────────────────────────────

func printMan() {
	fmt.Print(`
NAME
    pan211x-config - generate PAN211x register init tables

SYNOPSIS
    pan211x-config [options]

DESCRIPTION
    Emits two Go slices (InitP1, InitP0) containing only the register writes
    that differ from the chip power-on defaults, ready to paste into the
    pan211x driver.  Apply InitP1 with PAGE_CFG=1 before RF calibration
    (step 4 of Init), then InitP0 with PAGE_CFG=0 after OTP read (step 5).

GLOBAL OPTIONS
    --mode  xn297l|fs01|fs32|ble
            Chip operating mode.  (default: xn297l)

    --rate  1m|2m|250k
            Air data rate.  BLE always uses 1 Mbps unless --s2s8 is set.
            (default: 1m)

    --power <dBm>
            TX output power.  Valid levels: -25, -18, -12, -6, 0, 3, 6, 9, 11.
            Use 99 for the 0 dBm low-power analog variant.  (default: 9)

    --xtal  16|32
            Crystal frequency in MHz.  (default: 16)

    --iface i2c|spi3|spi4
            Host interface.  (default: spi3)

    --rx-gain low|high
            RX LNA gain preset.  (default: low)

    --agc
            Enable automatic gain control.

    --iomux
            Route IRQ through IO MUX.

    --irq   <0-255>
            Interrupt mask register value; 255 enables all interrupts.
            (default: 255)

    --rx-limit
            Enable RX payload length limiting.

    --pkg   <name>
            Go package name written into the output file.  (default: pan211x)

    --man
            Print this help and exit.

XN297L / FS01 / FS32 OPTIONS
    --addr  <2-5>
            Address width in bytes.  (default: 5)

    --crc   off|1|2
            CRC length.  (default: 2)

    --white
            Enable data whitening / scrambling.  (default: true)

    --dpl
            Enable dynamic payload length (Enhance mode).

    --work-mode  normal|enhance
            Packet mode.  Enhance adds ACK and auto-retransmit.
            (default: normal)

    --no-ack
            TX no-ACK flag; skip waiting for ACK after each packet.
            (default: true)

    --ack-timeout  <µs>
            RX / ACK wait timeout.  (default: 2000)

    --auto-delay  <µs>
            Auto-retransmit inter-packet delay (Enhance mode).  (default: 250)

    --auto-count  <0-15>
            Auto-retransmit attempt limit (Enhance mode).  (default: 3)

    --trx-delay  <µs>
            TX→RX turnaround delay (Enhance mode).  (default: 0)

    --tx-dev  250k|300k
            XN297L 1 Mbps TX frequency deviation.  250k gives narrower
            spectrum; 300k improves compatibility with NRF24.  (default: 250k)

FS32-ONLY OPTIONS
    --endian  big|little
            Byte order of the over-the-air payload.  (default: big)

    --crc-skip
            Skip CRC address bytes.

BLE OPTIONS
    --s2s8  off|s2|s8
            BLE long-range coded PHY mode.  (default: off)

    --ble-head0  <hex byte>
            TXHDR0 PDU type byte inserted before the payload.
            0x42 = ADV_NONCONN_IND with TxAdd=1.  (default: 42)

    --ble-head1  <hex byte>
            TXHDR1 byte.  (default: 00)

    --ble-head-num  0|1|2
            Number of BLE header bytes prepended automatically.  (default: 2)

    --white-init  <value>
            BLE whitening seed (channel index | 0x40, pre-bit-reverse).
            83 = CH37 (2402 MHz), 51 = CH38 (2426 MHz), 115 = CH39 (2480 MHz).
            (default: 83)

    --len-filter  off|equal|exceed|beneath
            BLE payload length filter applied to received packets.
            (default: equal)

EXAMPLES
    XN297L normal TX, 1 Mbps, +9 dBm, 32 MHz crystal, SPI-3:
        pan211x-config --mode xn297l --rate 1m --power 9

    XN297L enhance mode, 2 Mbps, SPI-4:
        pan211x-config --mode xn297l --rate 2m --work-mode enhance \
            --dpl --no-ack=false --iface spi4

    BLE advertising beacon, 16 MHz crystal, I2C:
        pan211x-config --mode ble --iface i2c

    BLE long-range S8, +6 dBm:
        pan211x-config --mode ble --power 6 --s2s8 s8
`)
}

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	mode    := flag.String("mode", "xn297l", "chip mode: xn297l, fs01, fs32, ble")
	rate    := flag.String("rate", "1m", "data rate: 1m, 2m, 250k")
	power   := flag.Int("power", 9, "TX power dBm; 99 = 0dBm low-power variant")
	xtal    := flag.Int("xtal", 16, "crystal MHz: 16 or 32")
	iface   := flag.String("iface", "spi3", "interface: i2c, spi3, spi4")
	rxGain  := flag.String("rx-gain", "low", "RX gain: low, high")
	agc     := flag.Bool("agc", false, "enable AGC")
	iomux   := flag.Bool("iomux", false, "enable IO MUX IRQ")
	irqMask := flag.Int("irq", 255, "interrupt mask 0–255 (255 = all enabled)")
	rxLimit := flag.Bool("rx-limit", false, "enable RX length limit")
	txDev   := flag.String("tx-dev", "250k", "XN297L 1M TX deviation: 250k or 300k")
	// XN297L / FS
	addrW    := flag.Int("addr", 5, "address width bytes 2–5 (XN297L/FS)")
	crcStr   := flag.String("crc", "2", "CRC: off, 1, 2 (XN297L/FS)")
	white    := flag.Bool("white", true, "enable whitening/scrambling (XN297L/FS)")
	dpl      := flag.Bool("dpl", false, "enable dynamic payload length (XN297L/FS)")
	workMode := flag.String("work-mode", "normal", "normal or enhance (XN297L/FS)")
	noAck    := flag.Bool("no-ack", true, "TX no-ACK mode (default true)")
	ackTO    := flag.Int("ack-timeout", 2000, "ACK/RX timeout µs")
	autoD    := flag.Int("auto-delay", 250, "auto-retrans delay µs (Enhance)")
	autoC    := flag.Int("auto-count", 3, "auto-retrans count 0–15 (Enhance)")
	trxD     := flag.Int("trx-delay", 0, "TRx transition delay µs (Enhance)")
	endian   := flag.String("endian", "big", "endian: big, little (FS32 only)")
	crcSkip  := flag.Bool("crc-skip", false, "CRC skip address (FS32 only)")
	// BLE
	s2s8     := flag.String("s2s8", "off", "BLE long-range: off, s2, s8")
	bleHead0 := flag.String("ble-head0", "42", "BLE TXHDR0 hex byte (default 42 = ADV_NONCONN_IND|TxAdd)")
	bleHead1 := flag.String("ble-head1", "00", "BLE TXHDR1 hex byte")
	bleHeadN := flag.Int("ble-head-num", 2, "BLE header count: 0, 1, 2")
	whInit   := flag.Int("white-init", 83, "BLE whitening seed (83=CH37, 51=CH38, 115=CH39)")
	lenFilt  := flag.String("len-filter", "equal", "BLE length filter: off, equal, exceed, beneath")
	pkg      := flag.String("pkg", "pan211x", "Go package name")
	man      := flag.Bool("man", false, "print detailed help and exit")
	flag.Parse()

	if *man {
		printMan()
		return
	}

	// ── validate and convert ─────────────────────────────────────────────────

	var cfg Config
	cfg.Pkg = *pkg

	switch strings.ToLower(*mode) {
	case "xn297l", "xn297":
		cfg.ChipMode = chipXN297
	case "fs01":
		cfg.ChipMode = chipFS01
	case "fs32":
		cfg.ChipMode = chipFS32
	case "ble":
		cfg.ChipMode = chipBLE
	default:
		fmt.Fprintf(os.Stderr, "error: --mode must be xn297l, fs01, fs32, or ble\n")
		os.Exit(1)
	}

	switch strings.ToLower(*rate) {
	case "1m", "1mbps":
		cfg.DataRate = dr1M
	case "2m", "2mbps":
		cfg.DataRate = dr2M
	case "250k", "250kbps":
		cfg.DataRate = dr250K
	default:
		fmt.Fprintf(os.Stderr, "error: --rate must be 1m, 2m, or 250k\n")
		os.Exit(1)
	}
	if cfg.DataRate == dr2M && *xtal == 16 {
		fmt.Fprintf(os.Stderr, "error: 2 Mbps requires 32 MHz crystal\n")
		os.Exit(1)
	}

	pwr, ok := findPower(*power)
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unsupported --power %d dBm\n", *power)
		fmt.Fprintf(os.Stderr, "supported: 11 9 8 7 6 5 4 3 2 1 0 99 -1 -2 -5 -7 -8 -10 -11 -12 -14 -16 -19 -23 -25 -28 -33 -37 -40\n")
		os.Exit(1)
	}
	_ = pwr
	cfg.TxPowerDBm = *power

	if *xtal != 16 && *xtal != 32 {
		fmt.Fprintf(os.Stderr, "error: --xtal must be 16 or 32\n")
		os.Exit(1)
	}
	cfg.XtalMHz = *xtal

	switch strings.ToLower(*iface) {
	case "i2c":
		cfg.Iface = ifI2C
	case "spi3":
		cfg.Iface = ifSPI3
	case "spi4":
		cfg.Iface = ifSPI4
	default:
		fmt.Fprintf(os.Stderr, "error: --iface must be i2c, spi3, or spi4\n")
		os.Exit(1)
	}

	switch strings.ToLower(*rxGain) {
	case "low":
		cfg.RxGain = rxLowGain
	case "high":
		cfg.RxGain = rxHighGain
	default:
		fmt.Fprintf(os.Stderr, "error: --rx-gain must be low or high\n")
		os.Exit(1)
	}

	cfg.EN_AGC = *agc
	cfg.IOMUX_EN = *iomux
	cfg.IntMask = uint8(*irqMask)
	cfg.EnRxLimit = *rxLimit

	switch strings.ToLower(*txDev) {
	case "250k":
		cfg.TxDevSel = txDev250K
	case "300k":
		cfg.TxDevSel = txDev300K
	default:
		fmt.Fprintf(os.Stderr, "error: --tx-dev must be 250k or 300k\n")
		os.Exit(1)
	}

	// XN297L / FS parameters
	if *addrW < 2 || *addrW > 5 {
		fmt.Fprintf(os.Stderr, "error: --addr must be 2–5\n")
		os.Exit(1)
	}
	cfg.AddrWidth = uint8(*addrW)

	switch strings.ToLower(*crcStr) {
	case "off", "0":
		cfg.Crc = crcOff
	case "1":
		cfg.Crc = crc1
	case "2":
		cfg.Crc = crc2
	default:
		fmt.Fprintf(os.Stderr, "error: --crc must be off, 1, or 2\n")
		os.Exit(1)
	}
	cfg.EnWhite = *white
	cfg.EnDPL = *dpl

	switch strings.ToLower(*workMode) {
	case "normal":
		cfg.WorkMode = wmNormal
	case "enhance":
		cfg.WorkMode = wmEnhance
	default:
		fmt.Fprintf(os.Stderr, "error: --work-mode must be normal or enhance\n")
		os.Exit(1)
	}

	cfg.EnTxNoAck = *noAck
	cfg.RxTimeoutUs = uint16(*ackTO)
	cfg.AutoDelayUs = uint16(*autoD)
	cfg.AutoMaxCnt = uint8(*autoC)
	cfg.TRxDelayUs = uint16(*trxD)

	switch strings.ToLower(*endian) {
	case "big":
		cfg.Endian = endianBig
	case "little":
		cfg.Endian = endianLittle
	default:
		fmt.Fprintf(os.Stderr, "error: --endian must be big or little\n")
		os.Exit(1)
	}
	cfg.CrcSkipAddr = *crcSkip

	// BLE parameters
	switch strings.ToLower(*s2s8) {
	case "off", "dis":
		cfg.S2S8Mode = s2s8Dis
	case "s2":
		cfg.S2S8Mode = s2s8S2
	case "s8":
		cfg.S2S8Mode = s2s8S8
	default:
		fmt.Fprintf(os.Stderr, "error: --s2s8 must be off, s2, or s8\n")
		os.Exit(1)
	}

	h0, err := hex.DecodeString(strings.TrimPrefix(*bleHead0, "0x"))
	if err != nil || len(h0) != 1 {
		fmt.Fprintf(os.Stderr, "error: --ble-head0 must be a 1-byte hex value (e.g. 42)\n")
		os.Exit(1)
	}
	cfg.BLEHead0 = h0[0]

	h1, err := hex.DecodeString(strings.TrimPrefix(*bleHead1, "0x"))
	if err != nil || len(h1) != 1 {
		fmt.Fprintf(os.Stderr, "error: --ble-head1 must be a 1-byte hex value (e.g. 00)\n")
		os.Exit(1)
	}
	cfg.BLEHead1 = h1[0]

	if *bleHeadN < 0 || *bleHeadN > 2 {
		fmt.Fprintf(os.Stderr, "error: --ble-head-num must be 0, 1, or 2\n")
		os.Exit(1)
	}
	cfg.BLEHeadNum = *bleHeadN

	if *whInit < 0 || *whInit > 127 {
		fmt.Fprintf(os.Stderr, "error: --white-init must be 0–127\n")
		os.Exit(1)
	}
	cfg.WhiteInit = uint8(*whInit)

	switch strings.ToLower(*lenFilt) {
	case "off", "disable":
		cfg.LenFilterMode = bleLenDisable
	case "equal":
		cfg.LenFilterMode = bleLenEqual
	case "exceed":
		cfg.LenFilterMode = bleLenExceed
	case "beneath":
		cfg.LenFilterMode = bleLenBeneath
	default:
		fmt.Fprintf(os.Stderr, "error: --len-filter must be off, equal, exceed, or beneath\n")
		os.Exit(1)
	}
	cfg.WhiteListLen = 5 // default 5-byte whitelist (all 0xCC)
	for i := range cfg.WhiteList {
		cfg.WhiteList[i] = 0xCC
	}

	cfg.TxMode = 0 // single
	cfg.RxMode = 2 // continuous

	defaults := newRegState()
	rs := newRegState()
	setupConfig(rs, cfg)

	printGoFile(rs, defaults, cfg)
}
