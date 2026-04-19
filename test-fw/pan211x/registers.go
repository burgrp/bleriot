package pan211x

// PAN211x register definitions.
// Addresses are 7-bit. On I2C the access byte is: reg<<1 | R/W (0=write, 1=read).
// Notation: registers marked (P0) are Page 0 only; (P1) are Page 1 only;
// (shared) are accessible from either page.

// ── Page 0 register addresses ────────────────────────────────────────────────

const (
	// PAGE_CFG selects the active register bank. (shared)
	// 0x00 = Page 0 (user registers); 0x01 = Page 1 (analog RF / calibration).
	// Always restore to 0x00 after any Page 1 access.
	PAGE_CFG = uint8(0x00)

	// TRX_FIFO is the TX/RX FIFO access point. (P0)
	// Burst-write loads the TX FIFO; burst-read drains the RX FIFO.
	TRX_FIFO = uint8(0x01)

	// STATE_CFG controls the operating state machine. (shared)
	// Write STATE_* values. Must be in STB3 before changing configuration registers.
	STATE_CFG = uint8(0x02)

	// SYS_CFG: system control and soft reset. (P0)
	// bit[1] = SOFT_RSTL (active low); bit[2] = IRQ_DATA_MUX_EN.
	SYS_CFG = uint8(0x03)

	// SPI_CFG: SPI/I2C bus interface configuration. (P0)
	// bit[7] = REG_SPI3_REN (must be set before entering STB3).
	// On Page 1 this address maps to P1_OTP_DATA.
	SPI_CFG = uint8(0x04)

	// XTAL_CFG: crystal load-capacitor trim. (P0)
	// Write (OTP_word4 >> 4) | 0xC0. On Page 1 this address maps to P1_OTP_CTL.
	XTAL_CFG = uint8(0x05)

	// I2C_CFG: I2C interface configuration. (P0)
	// Default 0x05 (reserved bits 2:0 = 0b101 must be preserved).
	I2C_CFG = uint8(0x06)

	// WMODE_CFG0: protocol, CRC, whitening, endianness. (P0)
	// [7:6]=CRC_MODE [5:4]=WORK_MODE [3]=WHITEN_EN [2]=CRC_SKIP_ADDR [1]=TX_NOACK [0]=ENDIAN
	WMODE_CFG0 = uint8(0x07)

	// WMODE_CFG1: FIFO size, dynamic payload, enhanced mode, address width. (P0)
	// [7]=RX_GOON [6]=PRI_EXIT_RX [5]=FIFO_128_EN [4]=DPY_EN [3]=ENHANCE [1:0]=ADDR_BYTE_LEN
	WMODE_CFG1 = uint8(0x08)

	// RXPLLEN_CFG: fixed RX payload length in bytes. (P0)
	// Ignored when DPY_EN=1.
	RXPLLEN_CFG = uint8(0x09)

	// TXPLLEN_CFG: number of bytes to transmit from FIFO. (P0)
	// Must be written before every TX if length varies.
	TXPLLEN_CFG = uint8(0x0A)

	// RFIRQ_CFG: interrupt mask. (P0)
	// 0 = interrupt enabled; 1 = masked. Use IRQ_* bit constants.
	RFIRQ_CFG = uint8(0x0B)

	// PID_CFG: PID manual control and address-error threshold. (P0)
	// [7]=PID_MANUAL_EN [6:4]=ADDR_ERR_THR [3:2]=RX_PID_MANUAL [1:0]=TX_PID_MANUAL
	PID_CFG = uint8(0x0C)

	// TRXTWTL_CFG: TX<->RX switch wait time, bits [7:0]. (P0)
	TRXTWTL_CFG = uint8(0x0D)

	// TRXTWTH_CFG: TX<->RX switch wait time, bits [14:8]. (P0)
	TRXTWTH_CFG = uint8(0x0E)

	// PIPE0_RXADDR0–4: 5-byte hardware RX address filter for pipe 0. (P0)
	// Byte 0 is the LSB (first byte on air). Default 0xCC×5.
	PIPE0_RXADDR0 = uint8(0x0F)
	PIPE0_RXADDR1 = uint8(0x10)
	PIPE0_RXADDR2 = uint8(0x11)
	PIPE0_RXADDR3 = uint8(0x12)
	PIPE0_RXADDR4 = uint8(0x13)

	// TXADDR0–4: TX destination address, same byte order as PIPE0_RXADDR. (P0)
	// Must match the receiver's PIPE0_RXADDR for hardware filtering to pass.
	TXADDR0 = uint8(0x14)
	TXADDR1 = uint8(0x15)
	TXADDR2 = uint8(0x16)
	TXADDR3 = uint8(0x17)
	TXADDR4 = uint8(0x18)

	// PKT_EXT_CFG: auto-insert header bytes, FEC / spread-spectrum. (P0)
	// [6]=HDR_LEN_EXIST [5:4]=HDR_LEN_NUMB [3]=PRI_TX_FEC [2]=PRI_RX_FEC [1:0]=PRI_CI_MODE
	PKT_EXT_CFG = uint8(0x19)

	// WHITEN_CFG: whitening LFSR seed and skip-address flag. (P0)
	// [7]=WHITEN_SKIP_ADDR [6:0]=WHITEN_SEED. Use WHITEN_* constants.
	WHITEN_CFG = uint8(0x1A)

	// TXHDR0_CFG: auto-inserted TX header byte 0. (P0)
	// Only used when PKT_EXT_CFG.HDR_LEN_EXIST=1.
	// On Page 1 this address maps to P1_CAL_CTL.
	TXHDR0_CFG = uint8(0x1B)

	// TXHDR1_CFG: auto-inserted TX header byte 1. (P0)
	TXHDR1_CFG = uint8(0x1C)

	// TXRAMADDR_CFG: TX FIFO RAM start offset (normally 0x00). (P0)
	TXRAMADDR_CFG = uint8(0x1D)

	// RXRAMADDR_CFG: RX FIFO RAM start offset (normally 0x00). (P0)
	RXRAMADDR_CFG = uint8(0x1E)

	// RXPIPE_CFG: enable bits for RX pipes 0–5. (P0)
	// Use PIPE*_EN bit constants. Default 0x01 (pipe 0 only).
	RXPIPE_CFG = uint8(0x1F)

	// PIPE1_RXADDR0–4: 5-byte RX address for pipe 1. (P0)
	PIPE1_RXADDR0 = uint8(0x20)
	PIPE1_RXADDR1 = uint8(0x21)
	PIPE1_RXADDR2 = uint8(0x22)
	PIPE1_RXADDR3 = uint8(0x23)
	PIPE1_RXADDR4 = uint8(0x24)

	// PIPE2–5_RXADDR0: LSB of pipes 2–5 address. (P0)
	// MSBs are shared with pipe 1.
	PIPE2_RXADDR0 = uint8(0x25)
	PIPE3_RXADDR0 = uint8(0x26)
	PIPE4_RXADDR0 = uint8(0x27)
	PIPE5_RXADDR0 = uint8(0x28)

	// TXAUTO_CFG: auto-retransmit delay and count (enhanced mode only). (P0)
	// [7:4]=ARD (delay = 250µs×(ARD+1)) [3:0]=ARC (0=off, max 14).
	TXAUTO_CFG = uint8(0x29)

	// TRXMODE_CFG: TX/RX mode selection and pre-sync options. (P0)
	// [7]=TX_MODE [6:5]=RX_MODE [0]=PRE_SYNC_EN (must stay 1).
	TRXMODE_CFG = uint8(0x2A)

	// RXTIMEOUTL_CFG / RXTIMEOUTH_CFG: 16-bit RX timeout in µs. (P0)
	// Default 0x07D0 = 2000 µs. Write low byte first.
	RXTIMEOUTL_CFG = uint8(0x2B)
	RXTIMEOUTH_CFG = uint8(0x2C)

	// BLEMATCH_CFG0: BLE sniffer, whitelist filter, length filter. (P0)
	// [7]=SNIF_EN [6:4]=WL_MATCH_MODE [3:2]=BLELEN_MATCH_MODE
	BLEMATCH_CFG0 = uint8(0x2D)

	// BLEMATCH_CFG1: reserved — always write 0x28, never modify. (P0)
	BLEMATCH_CFG1 = uint8(0x2E)

	// WLIST0–5_CFG: 6-byte BLE whitelist AdvA (byte 0 = bits[7:0]). (P0)
	WLIST0_CFG = uint8(0x2F)
	WLIST1_CFG = uint8(0x30)
	WLIST2_CFG = uint8(0x31)
	WLIST3_CFG = uint8(0x32)
	WLIST4_CFG = uint8(0x33)
	WLIST5_CFG = uint8(0x34)

	// BLEMATCHSTART_CFG: byte offset in the received packet where whitelist
	// comparison begins. Default 0x07. (P0)
	BLEMATCHSTART_CFG = uint8(0x35)

	// RF_DATARATE_CFG: air data rate. (P0)
	// Reserved bits [7:6]=01 and [3:0]=0101 must be included. Use DATARATE_* constants.
	RF_DATARATE_CFG = uint8(0x36)

	// RF_CHANNEL_CFG: RF channel. (P0)
	// Center frequency = 2400 + RF_CHANNEL_CFG [MHz]. Range 0–83.
	RF_CHANNEL_CFG = uint8(0x39)

	// IRQ_MUX_CFG: IRQ pin function selection. (P0)
	// [3:2]=OCLK_SEL [1:0]=IRQ_MUX (00=IRQ, 01=clock out, 10=PA ctrl).
	IRQ_MUX_CFG = uint8(0x45)

	// MISC_CFG: ACK pipe, IRQ polarity, PID mode. (P0)
	// [4]=PID_LOW_SEL [3]=IRQ_HIGH_EN [2:0]=ACK_PIPE.
	MISC_CFG = uint8(0x6F)

	// RFIRQFLG: interrupt status flags (write 1 to clear). (P0)
	// Use IRQ_* bit constants.
	RFIRQFLG = uint8(0x73)

	// STATUS0: RX pipe number and PID (read-only). (P0)
	// [6:4]=RX_SYNC_ADDR (pipe 0–5; 7=FIFO empty) [3:2]=RX_PID [1:0]=TX_PID.
	STATUS0 = uint8(0x74)

	// STATUS1: received header byte 0 (read-only). (P0)
	// Valid after RX_IRQ. In BLE mode = PDU type byte.
	STATUS1 = uint8(0x75)

	// STATUS2: received header byte 1 (read-only). (P0)
	// In BLE mode = PDU length byte.
	STATUS2 = uint8(0x76)

	// STATUS3: received payload length in bytes (read-only). (P0)
	STATUS3 = uint8(0x77)

	// PKT_RSSI_L / PKT_RSSI_H: 14-bit RSSI of the last received packet. (P0)
	PKT_RSSI_L = uint8(0x7A)
	PKT_RSSI_H = uint8(0x7B)

	// RT_RSSI_L / RT_RSSI_H: 14-bit real-time ambient noise RSSI. (P0)
	// On Page 1, 0x7F maps to P1_CAL_STATUS_DONE instead.
	RT_RSSI_L = uint8(0x7E)
	RT_RSSI_H = uint8(0x7F)
)

// ── Undocumented Page 0 RF analog tuning registers ───────────────────────────
// Written during Init from the SDK ES_Tool V1.2.6 sequence (16 MHz crystal).
// Names are address-based because the registers are not described in the RM.
// Do not modify without guidance from PANCHIP.

const (
	RF_ANA_43 = uint8(0x43) // RF driver tuning; also TX-power dependent
	RF_ANA_44 = uint8(0x44) // RF output tuning; TX-power dependent
	RF_ANA_55 = uint8(0x55)
	RF_ANA_56 = uint8(0x56)
	RF_ANA_57 = uint8(0x57)
	RF_ANA_5A = uint8(0x5A)
	RF_ANA_5B = uint8(0x5B)
	RF_ANA_5C = uint8(0x5C)
	RF_ANA_5D = uint8(0x5D) // 0xDC normal; 0xD4 in high-gain RX mode
	RF_ANA_5E = uint8(0x5E)
	RF_ANA_5F = uint8(0x5F)
	RF_ANA_60 = uint8(0x60)
	RF_ANA_61 = uint8(0x61) // 0x2E normal; 0x3E in high-gain RX mode
	RF_ANA_66 = uint8(0x66)
	RF_ANA_68 = uint8(0x68)
	RF_ANA_6E = uint8(0x6E)
)

// ── Page 1 register addresses ─────────────────────────────────────────────────
// Select Page 1 with PAGE_CFG = 0x01. Addresses below access different physical
// registers than their Page 0 counterparts. Always restore PAGE_CFG = 0x00
// before returning to normal operation.

const (
	// P1_OTP_DATA: OTP calibration data register. Dual-use with SPI_CFG (0x04).
	// Write 0x04 → read word 2 (value2). Write 0x08 → read word 4 (value4).
	P1_OTP_DATA = uint8(0x04)

	// P1_OTP_CTL: OTP mode control. Dual-use with XTAL_CFG (0x05).
	// Write OTP_CTL_START before reading, OTP_CTL_STOP after.
	P1_OTP_CTL = uint8(0x05)

	// P1_CAL_CTL: calibration trigger (one-hot). Dual-use with TXHDR0_CFG (0x1B).
	// Write CAL_* values in sequence. Poll P1_CAL_STATUS_* for completion.
	P1_CAL_CTL = uint8(0x1B)

	// P1_RF_TUNE_27: RF analog tuning, init value 0xAA.
	P1_RF_TUNE_27 = uint8(0x27)

	// P1_RF_TUNE_32 / P1_RF_TUNE_33: RF analog tuning, init 0x1E / 0x19.
	P1_RF_TUNE_32 = uint8(0x32)
	P1_RF_TUNE_33 = uint8(0x33)

	// P1_RF_TUNE_37: RF analog tuning, init value 0x15.
	P1_RF_TUNE_37 = uint8(0x37)

	// P1_RF_TUNE_3A: RF analog tuning, init value 0x14.
	P1_RF_TUNE_3A = uint8(0x3A)

	// P1_TX_PWR_AMP: TX power amplitude control.
	// 0x13 = 0 dBm; 0x17 = 9 dBm.
	P1_TX_PWR_AMP = uint8(0x3C)

	// P1_RF_TUNE_3E: RF analog tuning, init value 0xF1.
	P1_RF_TUNE_3E = uint8(0x3E)

	// P1_VCO_PA_CTL: VCO/PA control.
	// 0xA2 = normal; 0x20 = enter carrier-wave; 0x00 = exit carrier-wave.
	P1_VCO_PA_CTL = uint8(0x41)

	// P1_CW_TUNE: carrier-wave mode tuning.
	// 0x4E = carrier-wave active; 0x00 = normal.
	P1_CW_TUNE = uint8(0x42)

	// P1_PA_TUNE_43: OTP-dependent PA tuning. Write 0x10|(calBit).
	P1_PA_TUNE_43 = uint8(0x43)

	// P1_PA_BIAS: PA bias control.
	// 0xB0 = 9 dBm; 0xBD = 0 dBm.
	P1_PA_BIAS = uint8(0x46)

	// P1_PA_TUNE_47: OTP-dependent PA tuning. Write 0x83|((value2>>1)&0x70).
	P1_PA_TUNE_47 = uint8(0x47)

	// P1_TX_PWR_CTL: TX power control register.
	// 0x88 for both 0 dBm and 9 dBm.
	P1_TX_PWR_CTL = uint8(0x48)

	// P1_RF_TUNE_4C: RF analog tuning, init value 0x48.
	P1_RF_TUNE_4C = uint8(0x4C)

	// P1_CAL_STATUS_PHASE1: calibration status.
	// Bit [7] = 1 when phase calibration 1 is complete.
	P1_CAL_STATUS_PHASE1 = uint8(0x6D)

	// P1_CAL_STATUS_VCO: calibration status.
	// Bit [6] = 1 when VCO calibration is complete.
	P1_CAL_STATUS_VCO = uint8(0x70)

	// P1_CAL_STATUS_DONE: calibration status. Dual-use with RT_RSSI_H (0x7F).
	// Bit [7] = 1 when frequency or phase 2 calibration is complete.
	P1_CAL_STATUS_DONE = uint8(0x7F)
)

// ── STATE_CFG operation codes ─────────────────────────────────────────────────

const (
	// STATE_STB3: Standby 3 with EN_LS_3V=1 (bit 6). Primary idle state.
	// Enter before modifying any configuration register.
	STATE_STB3 = uint8(0x74)

	// STATE_TX: Transmit mode with EN_LS_3V=1.
	STATE_TX = uint8(0x75)

	// STATE_RX: Receive mode with EN_LS_3V=1.
	STATE_RX = uint8(0x76)

	// STATE_SLEEP: enter low-power sleep (register contents retained).
	// Sequence: write STATE_STB3, then STATE_SLEEP.
	STATE_SLEEP = uint8(0x21)

	// STATE_WAKE: exit sleep. Follow with STATE_STB3 and 1 ms crystal delay.
	STATE_WAKE = uint8(0x22)
)

// ── RFIRQFLG / RFIRQ_CFG bit constants ───────────────────────────────────────
// Used in both the mask register (RFIRQ_CFG) and the flag register (RFIRQFLG).
// In RFIRQ_CFG: 0 = interrupt enabled, 1 = masked.
// In RFIRQFLG: write 1 to clear the flag.

const (
	IRQ_TX         = uint8(0x80) // TX complete (RF_IT_TX_IRQ)
	IRQ_MAX_RT     = uint8(0x40) // max retransmits reached (RF_IT_MAX_RT_IRQ)
	IRQ_ADDR_ERR   = uint8(0x20) // address match error (RF_IT_ADDR_ERR_IRQ)
	IRQ_CRC_ERR    = uint8(0x10) // CRC error (RF_IT_CRC_ERR_IRQ)
	IRQ_LEN_ERR    = uint8(0x08) // payload length error (RF_IT_LEN_ERR_IRQ)
	IRQ_PID_ERR    = uint8(0x04) // duplicate PID (RF_IT_PID_ERR_IRQ)
	IRQ_RX_TIMEOUT = uint8(0x02) // RX timeout (RF_IT_RX_TIMEOUT_IRQ)
	IRQ_RX         = uint8(0x01) // valid packet received (RF_IT_RX_IRQ)
	IRQ_ALL        = uint8(0xFF) // all flags (RF_IT_ALL_IRQ)
)

// ── SPI_CFG values ────────────────────────────────────────────────────────────

const (
	// SPI_CFG_INIT must be written before entering STB3.
	// REG_SPI3_REN=1 enables 3-wire SPI reads; 0b011 are reserved constant bits.
	// After soft-reset, reading SPI_CFG should return this value (chip-present check).
	SPI_CFG_INIT = uint8(0x83)
)

// ── SYS_CFG values ────────────────────────────────────────────────────────────

const (
	// SYS_CFG_RESET asserts SOFT_RSTL (active low). Delay 1 ms before release.
	SYS_CFG_RESET = uint8(0x00)

	// SYS_CFG_RELEASE releases SOFT_RSTL.
	SYS_CFG_RELEASE = uint8(0x02)

	// SYS_CFG_NORMAL sets IRQ_DATA_MUX_EN=1 and releases reset.
	// Written after OTP read is complete.
	SYS_CFG_NORMAL = uint8(0x06)
)

// ── WMODE_CFG0 field values ───────────────────────────────────────────────────

const (
	// CRC mode (bits [7:6]).
	CRC_OFF = uint8(0x00)
	CRC_1B  = uint8(0x40)
	CRC_2B  = uint8(0x80)
	CRC_3B  = uint8(0xC0) // required for BLE

	// WORK_MODE (bits [5:4]).
	WORK_MODE_XN297L = uint8(0x00)
	WORK_MODE_BLE    = uint8(0x30)

	// Individual control bits.
	WHITEN_EN_BIT    = uint8(0x08)
	CRC_SKIP_ADDR_BIT = uint8(0x04)
	TX_NOACK_BIT     = uint8(0x02)
	ENDIAN_BIG       = uint8(0x01) // XN297L-compatible
	ENDIAN_LITTLE    = uint8(0x00) // BLE
)

// ── WMODE_CFG1 field values ───────────────────────────────────────────────────

const (
	RX_GOON_BIT    = uint8(0x80) // stay in RX after packet received
	PRI_EXIT_RX_BIT = uint8(0x40) // force exit RX
	FIFO_128_BIT   = uint8(0x20) // 128-byte FIFO (vs 64-byte)
	DPY_EN_BIT     = uint8(0x10) // dynamic payload length
	ENHANCE_BIT    = uint8(0x08) // enhanced mode (auto-ACK, PID)

	// Address width (bits [1:0]).
	ADDR_2B = uint8(0x00)
	ADDR_3B = uint8(0x01)
	ADDR_4B = uint8(0x02)
	ADDR_5B = uint8(0x03)
)

// ── PKT_EXT_CFG bit constants ─────────────────────────────────────────────────

const (
	// HDR_LEN_EXIST enables auto-insertion of header bytes before the payload.
	// When set, FIFO must contain only payload (no header prefix).
	HDR_LEN_EXIST_BIT = uint8(0x40)

	// HDR_LEN_NUMB selects how many header bytes to insert (bits [5:4]).
	HDR_LEN_1_BIT = uint8(0x10) // insert 1 header byte
	HDR_LEN_2_BIT = uint8(0x20) // insert 2 header bytes

	// Spread-spectrum / FEC bits.
	PRI_TX_FEC_BIT = uint8(0x08)
	PRI_RX_FEC_BIT = uint8(0x04)
	PRI_CI_S2      = uint8(0x01)
	PRI_CI_S8      = uint8(0x02)

	// PKT_EXT_CFG_BLE: auto-insert 2-byte BLE header (PDU type + length).
	PKT_EXT_CFG_BLE = HDR_LEN_EXIST_BIT | HDR_LEN_2_BIT // 0x60
)

// ── WHITEN_CFG values ─────────────────────────────────────────────────────────

const (
	// WHITEN_SKIP_ADDR_BIT: whitening starts after the address field (bit [7]).
	// Not needed in BLE WORK_MODE; address skipping is automatic.
	WHITEN_SKIP_ADDR_BIT = uint8(0x80)

	// WHITEN_DEFAULT: XN297L-compatible whitening seed.
	WHITEN_DEFAULT = uint8(0x7F)

	// BLE advertising channel whitening seeds (WORK_MODE_BLE, no SKIP_ADDR bit).
	// Formula: bit_reverse7(BLE_channel_index | 0x40).
	WHITEN_BLE_CH37 = uint8(0x53) // BLE ch 37 / RF_CH 0x02 / 2402 MHz
	WHITEN_BLE_CH38 = uint8(0x33) // BLE ch 38 / RF_CH 0x1A / 2426 MHz
	WHITEN_BLE_CH39 = uint8(0x73) // BLE ch 39 / RF_CH 0x50 / 2480 MHz
)

// ── RXPIPE_CFG bit constants ──────────────────────────────────────────────────

const (
	PIPE0_EN = uint8(0x01)
	PIPE1_EN = uint8(0x02)
	PIPE2_EN = uint8(0x04)
	PIPE3_EN = uint8(0x08)
	PIPE4_EN = uint8(0x10)
	PIPE5_EN = uint8(0x20)
)

// ── TRXMODE_CFG bit constants ─────────────────────────────────────────────────

const (
	// TX_MODE bit [7]: 0 = single burst, 1 = continuous carrier.
	TX_SINGLE_BIT     = uint8(0x00)
	TX_CONTINUOUS_BIT = uint8(0x80)

	// RX_MODE bits [6:5] (normal mode).
	RX_SINGLE_BIT     = uint8(0x00)
	RX_TIMEOUT_BIT    = uint8(0x20) // single + timeout
	RX_CONTINUOUS_BIT = uint8(0x40)

	// PRE_SYNC_EN bit [0]: preamble detect. Default=1; must remain set.
	PRE_SYNC_EN_BIT = uint8(0x01)

	// TRXMODE_CFG_NORMAL: single TX, continuous RX, pre-sync enabled.
	TRXMODE_CFG_NORMAL = TX_SINGLE_BIT | RX_CONTINUOUS_BIT | PRE_SYNC_EN_BIT // 0x41
)

// ── RF_DATARATE_CFG values ────────────────────────────────────────────────────
// Reserved bits [7:6]=0b01 and [3:0]=0b0101 are included in each constant.

const (
	DATARATE_1MBPS   = uint8(0x55) // 1 Mbps  — required for BLE
	DATARATE_2MBPS   = uint8(0x65) // 2 Mbps
	DATARATE_250KBPS = uint8(0x75) // 250 kbps
)

// ── RF_CHANNEL_CFG notable values ─────────────────────────────────────────────

const (
	// RF_CH_CAL is used during Init calibration (2485 MHz, outside ISM channels).
	// Must not be changed; must differ from the operating channel.
	RF_CH_CAL = uint8(0x55)

	// BLE advertising channel RF_CH values (F = 2400 + RF_CH MHz).
	RF_CH_BLE_37 = uint8(0x02) // 2402 MHz
	RF_CH_BLE_38 = uint8(0x1A) // 2426 MHz
	RF_CH_BLE_39 = uint8(0x50) // 2480 MHz
)

// ── BLEMATCH_CFG0 bit constants ───────────────────────────────────────────────

const (
	SNIF_EN_BIT = uint8(0x80) // sniffer: accept all packets

	// WL_MATCH_MODE (bits [6:4]): whitelist filter depth.
	WL_MATCH_NONE = uint8(0x00)
	WL_MATCH_1B   = uint8(0x10) // compare bits [47:40]
	WL_MATCH_2B   = uint8(0x20) // compare bits [47:32]
	WL_MATCH_3B   = uint8(0x30) // compare bits [47:24]
	WL_MATCH_4B   = uint8(0x40) // compare bits [47:16]
	WL_MATCH_5B   = uint8(0x50) // compare bits [47:8]
	WL_MATCH_FULL = uint8(0x60) // compare full 48 bits

	// BLELEN_MATCH_MODE (bits [3:2]): length filter.
	BLELEN_DISABLED = uint8(0x00)
	BLELEN_EQUAL    = uint8(0x04)
	BLELEN_GT       = uint8(0x08)
	BLELEN_LT       = uint8(0x0C)
)

// ── MISC_CFG bit constants ────────────────────────────────────────────────────

const (
	IRQ_HIGH_EN_BIT = uint8(0x08) // IRQ pin polarity: 1=active high, 0=active low
	PID_LOW_SEL_BIT = uint8(0x10) // PID comparison mode; set in BLE RX mode
)

// ── IRQ_MUX_CFG values ────────────────────────────────────────────────────────

const (
	// IRQ_MUX selects IRQ pin function (bits [1:0]).
	IRQ_MUX_IRQ = uint8(0x00) // interrupt output (default)
	IRQ_MUX_CLK = uint8(0x01) // clock output
	IRQ_MUX_PA  = uint8(0x02) // PA control signal

	// OCLK_SEL clock frequency when IRQ_MUX=IRQ_MUX_CLK (bits [3:2]).
	OCLK_1KHZ  = uint8(0x00)
	OCLK_4KHZ  = uint8(0x04)
	OCLK_8MHZ  = uint8(0x08)
	OCLK_16MHZ = uint8(0x0C)
)

// ── STATUS0 field constants ───────────────────────────────────────────────────

const (
	STATUS0_PIPE_MASK  = uint8(0x70) // bits [6:4] = received pipe number
	STATUS0_PIPE_SHIFT = 4
	STATUS0_PIPE_EMPTY = uint8(0x70) // value when FIFO is empty (pipe=7)
)

// ── Page 1 calibration control values ─────────────────────────────────────────
// Write to P1_CAL_CTL in this exact order. Poll the corresponding status register
// for completion before advancing to the next step.

const (
	CAL_VCO     = uint8(0x08) // trigger VCO calibration; poll P1_CAL_STATUS_VCO bit[6]
	CAL_THERMAL = uint8(0x10) // trigger thermal calibration; mandatory 55 ms delay
	CAL_FREQ    = uint8(0x20) // trigger frequency calibration (chip must be in RX first); poll P1_CAL_STATUS_DONE bit[7]
	CAL_PHASE1  = uint8(0x40) // trigger phase calibration 1; poll P1_CAL_STATUS_PHASE1 bit[7]
	CAL_PHASE2  = uint8(0x80) // trigger phase calibration 2; poll P1_CAL_STATUS_DONE bit[7]
	CAL_STOP    = uint8(0x00) // stop all calibration

	CAL_VCO_DONE_BIT    = uint8(0x40) // P1_CAL_STATUS_VCO: VCO done
	CAL_PHASE1_DONE_BIT = uint8(0x80) // P1_CAL_STATUS_PHASE1: phase 1 done
	CAL_DONE_BIT        = uint8(0x80) // P1_CAL_STATUS_DONE: freq / phase 2 done
)

// ── Page 1 OTP constants ──────────────────────────────────────────────────────

const (
	OTP_CTL_START = uint8(0x00) // P1_OTP_CTL: enter OTP read mode
	OTP_CTL_STOP  = uint8(0x01) // P1_OTP_CTL: exit OTP read mode

	OTP_READ_WORD2 = uint8(0x04) // P1_OTP_DATA write command to read word 2 (value2)
	OTP_READ_WORD4 = uint8(0x08) // P1_OTP_DATA write command to read word 4 (value4)

	// OTP word 2 (value2) field masks.
	OTP_VALID_MASK = uint8(0x0F) // bits [3:0] must equal OTP_VALID_VAL
	OTP_VALID_VAL  = uint8(0x01)
	OTP_CAL_MASK   = uint8(0x10) // bit [4]: CAL_BIT → P1_PA_TUNE_43 bit [0]
	OTP_PA_TRIM_MASK = uint8(0x70) // bits [6:4] after >>1 → P1_PA_TUNE_47 bits [6:4]

	// OTP word 4 (value4) field masks.
	OTP_XTAL_MASK = uint8(0xF0) // bits [7:4]: crystal trim → XTAL_CFG upper nibble
)

// ── Page 1 TX power preset values ─────────────────────────────────────────────
// Apply together via the TX power sequence (see registers.md §TX Power Configuration).

const (
	// P1_TX_PWR_AMP values.
	TX_PWR_AMP_0DBM = uint8(0x13)
	TX_PWR_AMP_9DBM = uint8(0x17)

	// P1_PA_BIAS values.
	PA_BIAS_0DBM = uint8(0xBD)
	PA_BIAS_9DBM = uint8(0xB0)

	// P1_TX_PWR_CTL value (same for both power levels).
	TX_PWR_CTL_VAL = uint8(0x88)

	// RF_ANA_44 (Page 0) values for TX power.
	RF_ANA_44_0DBM = uint8(0x84)
	RF_ANA_44_9DBM = uint8(0x8C)
)
