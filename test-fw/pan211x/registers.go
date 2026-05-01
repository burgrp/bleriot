package pan211x

// PAN211x register definitions.
// Addresses are 7-bit. On I2C the access byte is: reg<<1 | R/W (0=write, 1=read).
// Notation: registers marked (P0) are Page 0 only; (P1) are Page 1 only;
// (shared) are accessible from either page.
//
// Bit-field names and functional descriptions come from the ES_Tool v2.x
// decompiled sources (genconfig_reg.pyc). Where the ES_Tool name differs from
// the SDK header name, both are noted.

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
	// ES_Tool calls this IPTAT_CFG (IPTAT bias current trim).
	XTAL_CFG = uint8(0x05)

	// I2C_CFG / LP_CFG: low-power and interface control register. (P0)
	// ES_Tool name: LP_CFG. See LP_CFG_* bit constants below.
	// Default 0x05 = EN_DIG_COREPOWER | PTAT_TEMP_TRIM(0b010).
	I2C_CFG = uint8(0x06)
	LP_CFG  = uint8(0x06) // ES_Tool alias for I2C_CFG

	// WMODE_CFG0: protocol, CRC, whitening, endianness. (P0)
	// [7:6]=CRC_MODE [5]=CHIP_MODE [4]=NORDIC_ENHANCE [3]=WHITEN_EN [2]=CRC_SKIP_ADDR [1]=TX_NOACK [0]=ENDIAN
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
	// [7]=W_RX_MAX_CTRL_EN [6]=HDR_LEN_EXIST [5:4]=HDR_LEN_NUMB [3]=PRI_TX_FEC [2]=PRI_RX_FEC [1:0]=PRI_CI_MODE
	PKT_EXT_CFG = uint8(0x19)

	// SCR_CFG / WHITEN_CFG: whitening LFSR seed and address-skip flag. (P0)
	// ES_Tool calls this SCR_CFG (scrambler config); same register.
	// [7]=ACCADDR_SCR_DIS (skip address in whitening) [6:0]=SCR_INI (seed).
	WHITEN_CFG = uint8(0x1A)
	SCR_CFG    = uint8(0x1A) // ES_Tool alias for WHITEN_CFG

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
	// [7:6]=RESERVED (must not be written) [5:0]=PIPE_EN_5..0
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
	// [7]=TX_CFG_MODE [6:5]=RX_CFG_MODE [4]=PRE_2BYTE_MODE [0]=PRE_SYNC_EN (must stay 1).
	TRXMODE_CFG = uint8(0x2A)

	// RXTIMEOUTL_CFG / RXTIMEOUTH_CFG: 16-bit RX timeout in µs. (P0)
	// Default 0x07D0 = 2000 µs. Write low byte first.
	RXTIMEOUTL_CFG = uint8(0x2B)
	RXTIMEOUTH_CFG = uint8(0x2C)

	// BLEMATCH_CFG0: BLE sniffer, whitelist filter, length filter. (P0)
	// [7]=SNIF_EN [6:4]=WL_MATCH_MODE [3:2]=BLELEN_MATCH_MODE [1:0]=PATT_MATCH_THRESHOLD[9:8]
	BLEMATCH_CFG0 = uint8(0x2D)

	// BLEMATCH_CFG1: pattern match threshold bits [7:0]. (P0)
	// Default 0x28. ES_Tool: never modify.
	BLEMATCH_CFG1 = uint8(0x2E)

	// WLIST0–5_CFG: 6-byte BLE whitelist AdvA (byte 0 = bits[7:0]). (P0)
	WLIST0_CFG = uint8(0x2F)
	WLIST1_CFG = uint8(0x30)
	WLIST2_CFG = uint8(0x31)
	WLIST3_CFG = uint8(0x32)
	WLIST4_CFG = uint8(0x33)
	WLIST5_CFG = uint8(0x34)

	// BLEMATCHSTART_CFG: packet byte offset where whitelist matching begins. (P0)
	// [7]=LQI_EN_2BYTE [5:0]=PLD_START_BYTE. Default 0x07.
	BLEMATCHSTART_CFG = uint8(0x35)

	// RF_DATARATE_CFG: air data rate. (P0)
	// [5:4]=BW_MODE (0=1Mbps, 1=2Mbps, 3=250kbps). Other bits are reserved.
	// Chip resets to 1 Mbps (full register value 0x45). Use DATARATE_REG_* constants.
	RF_DATARATE_CFG = uint8(0x36)

	// RF_OSC_CFG: oscillator select and demodulator zero-crossing offset. (P0)
	// [7]=OSC_SEL (1=external crystal, 0=internal RC) [6:0]=ZCONES_OFFSET.
	// Must be written before Page1 OTP read when using 16 MHz crystal.
	RF_OSC_CFG = uint8(0x37)

	// RF_ZC_CFG: demodulator zero-crossing normal shift and scale. (P0)
	// [6:5]=ZCNORMAL_SHIFT [4:0]=ZCONES_SCALE.
	RF_ZC_CFG = uint8(0x38)

	// RF_CHANNEL_CFG: RF channel. (P0)
	// Center frequency = 2400 + RF_CHANNEL_CFG [MHz]. Range 0–83.
	RF_CHANNEL_CFG = uint8(0x39)

	// RF_CH_OFFSET_CFG: channel frequency fine offset, bits [7:0]. (P0)
	RF_CH_OFFSET_CFG = uint8(0x3A)

	// RF_FREQ_OVR_L / _M / _H: 23-bit manual frequency offset override. (P0)
	// Only active when RF_FREQ_OVR_H.bit[7]=FREQUENCY_OFFSET_OVRDWD_SEL=1.
	RF_FREQ_OVR_L = uint8(0x3B) // bits [7:0]
	RF_FREQ_OVR_M = uint8(0x3C) // bits [15:8]
	RF_FREQ_OVR_H = uint8(0x3D) // [7]=OVR_SEL [6:0]=bits[22:16]

	// RF_SDNINT_CFG: sigma-delta notch interrupt config. (P0)
	// [6]=SDNINT_SEL [5:0]=SDNINT_OVRD.
	RF_SDNINT_CFG = uint8(0x3E)

	// RF_FREQ_CORR_RX_L / _H: 16-bit RX frequency correction word. (P0)
	// Loaded into demodulator. See PAN211_SetXTALFreq for 16 MHz values.
	RF_FREQ_CORR_RX_L = uint8(0x3F) // bits [7:0]
	RF_FREQ_CORR_RX_H = uint8(0x40) // bits [15:8]

	// RF_FREQ_CORR_TX_L / _H: 16-bit TX frequency correction word. (P0)
	// Used in carrier-wave mode to cancel crystal frequency deviation.
	RF_FREQ_CORR_TX_L = uint8(0x41) // bits [7:0]
	RF_FREQ_CORR_TX_H = uint8(0x42) // bits [15:8]

	// RF_PA_MODE_CFG: RF PA mode and RX ADC clock control. (P0)
	// [7]=RXADC_MODE_MANUAL_EN [6]=RXADC_MODE_SEL [5:4]=TXPA_MODE_SEL
	// [3]=EN_RXADCCLK [2]=FSYNVCO_TXCTK [1:0]=RXFLTR_IF
	// Written during data-rate configuration and TX power table application.
	RF_PA_MODE_CFG = uint8(0x43)

	// RF_PA_POUT_CFG: TX PA output current and LDO selection. (P0)
	// [7:4]=TXPA_POUT_CRNT [3:0]=TXPA_LDO_SEL. TX-power dependent.
	RF_PA_POUT_CFG = uint8(0x44)

	// IRQ_MUX_CFG: IRQ pin function selection. (P0)
	// [3:2]=OCLK_SEL [1:0]=IRQ_MUX (00=IRQ, 01=clock out, 10=PA ctrl).
	IRQ_MUX_CFG = uint8(0x45)

	// RF_AGC_CFG: AGC enable override and IQ filter mode. (P0)
	// [4]=AGC_EN_OVRD_SEL [3]=AGC_EN_OVRD [2:0]=AGC_IQFLTR_MODE.
	RF_AGC_CFG = uint8(0x46)

	// RF_AGC_FLTR_CFG: AGC filter override mode and coefficient. (P0)
	// [7:6]=AGC_FLTROVRD_MD [5:0]=AGC_IQFLTR_COEFF.
	RF_AGC_FLTR_CFG = uint8(0x47)

	// RF_AGC_WAIT_L: AGC wait time bits [7:0]. (P0)
	RF_AGC_WAIT_L = uint8(0x48)

	// RF_AGC_WAIT_H: AGC wait time bits [12:8]. (P0)
	// [4:0]=AGC_WAIT_TIME_12_8.
	RF_AGC_WAIT_H = uint8(0x49)

	// RF_AGC_SW_WAIT_L: AGC switch wait time bits [7:0]. (P0)
	RF_AGC_SW_WAIT_L = uint8(0x4A)

	// RF_AGC_SW_WAIT_H: AGC switch wait time bits [12:8] and switch control. (P0)
	// [6]=SWITCH_TMR_EN [5]=AGC_SWITCH_FREEZE [4:0]=AGC_WAIT_TIME_SWITCH_12_8.
	RF_AGC_SW_WAIT_H = uint8(0x4B)

	// RF_AGC_ITER_CFG: AGC freeze and iteration limit. (P0)
	// [7]=AGC_FREEZE [4]=AGC_ITERATION_FREEZE [3:0]=AGC_ITERATION_LIMIT.
	RF_AGC_ITER_CFG = uint8(0x4D)

	// RF_AGC_GAIN_CFG: AGC gain select and manual gain override. (P0)
	// [6]=GAIN_SEL (1=use GAIN_OVRD) [5:0]=GAIN_OVRD.
	// In non-AGC mode: write 0x7E (GAIN_SEL=1, GAIN_OVRD=62) as recommended baseline,
	// then SetRxGain adjusts GAIN_OVRD: 46 for low gain, 62 for high gain.
	RF_AGC_GAIN_CFG = uint8(0x4E)

	// RF_RSSI_AVG_CFG / RF_RSSI_AVG_CFG2: RSSI averaging config. (P0)
	RF_RSSI_AVG_CFG  = uint8(0x4F)
	RF_RSSI_AVG_CFG2 = uint8(0x51)

	// RF_RSSI_ERR_CFG: RSSI error flag enable and gap config. (P0)
	// [6]=RSSI_ERR_FLAG_EN [5:0]=RSSI_GAP_CFG.
	RF_RSSI_ERR_CFG = uint8(0x53)

	// RF_RSSI_THRESH: RSSI threshold for AGC switching. (P0)
	RF_RSSI_THRESH = uint8(0x54)

	// RF_RSSI_TH1–3: RSSI AGC threshold levels. (P0)
	// Written during Init for non-AGC mode. Values differ for AGC mode.
	RF_RSSI_TH1 = uint8(0x55)
	RF_RSSI_TH2 = uint8(0x56)
	RF_RSSI_TH3 = uint8(0x57)

	// RF_RSSI_INIT: RSSI initial value for AGC. (P0)
	RF_RSSI_INIT = uint8(0x58)

	// RF_RSSI_HYS: RSSI hysteresis for AGC. (P0)
	RF_RSSI_HYS = uint8(0x59)

	// RF_RSSI_FIX0–3: fixed RSSI calibration words. (P0)
	// Set by ES_Tool based on chip mode; not computed.
	RF_RSSI_FIX0 = uint8(0x5A)
	RF_RSSI_FIX1 = uint8(0x5B)
	RF_RSSI_FIX2 = uint8(0x5C)
	RF_RSSI_FIX3 = uint8(0x5D) // 0xDC=low-gain RX, 0xD4=high-gain RX

	// RF_GAIN_WORD0–3: AGC gain table entries. (P0)
	// Fixed calibration values. In high-gain RX mode RF_GAIN_WORD3 changes.
	RF_GAIN_WORD0 = uint8(0x5E)
	RF_GAIN_WORD1 = uint8(0x5F)
	RF_GAIN_WORD2 = uint8(0x60)
	RF_GAIN_WORD3 = uint8(0x61) // 0x2E low-gain, 0x3E high-gain RX

	// RF_RSSI_CONV: RSSI conversion gain. (P0)
	RF_RSSI_CONV = uint8(0x62)

	// RF_RSSI_BYPASS: RSSI bypass control. (P0)
	RF_RSSI_BYPASS = uint8(0x63)

	// RF_LDO_ANA_TIME: LDO analog setup/close timing. (P0)
	// [7:5]=LDO_ANA_CLOSE_TIME [4:0]=LDO_ANA_SETUP_TIME.
	RF_LDO_ANA_TIME = uint8(0x64)

	// RF_TX_PLL_TIME: TX RF PLL timing. (P0)
	// [6:4]=TX_RFPLL_CLOSE_TIME [3:0]=TX_RFPLL_SETUP_TIME.
	RF_TX_PLL_TIME = uint8(0x65)

	// RF_TX_ANA_TIME: TX analog setup time. (P0)
	// Written 0x34 during Init.
	RF_TX_ANA_TIME = uint8(0x66)

	// RF_RX_PLL_TIME: RX RF PLL timing. (P0)
	// [6:4]=RX_RFPLL_CLOSE_TIME [2:0]=TX_ANA_CLOSE_TIME.
	RF_RX_PLL_TIME = uint8(0x67)

	// RF_RX_PLL_SETUP: RX RF PLL setup time. (P0)
	// [5:0]=RX_RFPLL_SETUP_TIME. Written 0x0D during Init.
	RF_RX_PLL_SETUP = uint8(0x68)

	// RF_RX_ANA_TIME: RX analog close/setup timing. (P0)
	// [7:5]=RX_ANA_CLOSE_TIME [4:0]=RX_ANA_SETUP_TIME.
	RF_RX_ANA_TIME = uint8(0x69)

	// RF_EN_DIG_CFG: digital TX/RX and PLL enable override register. (P0)
	// [7]=TX_EN_DIG [6]=RX_EN_DIG [5]=TX_RFPLL_EN [4]=RX_RFPLL_EN
	// [3]=TX_EN_ANA [2]=RX_EN_ANA [1]=OSC_BUF_EN [0]=LDO_ANA_EN.
	// Written in carrier-wave entry/exit sequence.
	RF_EN_DIG_CFG = uint8(0x6A)

	// RF_PA_EN_CFG: PA enable and ramp control. (P0)
	// [7]=EN_PA [6]=EN_RAMP [5:0]=RAMP_DLY_TIME_UP.
	// Written in carrier-wave entry/exit sequence.
	RF_PA_EN_CFG = uint8(0x6B)

	// RF_PA_BUF_CFG: PA buffer enable and PA outside timing. (P0)
	RF_PA_BUF_CFG = uint8(0x6C)

	// RF_PA2_DLY: PA2 delay time down. (P0)
	RF_PA2_DLY = uint8(0x6D)

	// RF_PA_RAMP_DLY: PA ramp delay select. (P0)
	// [6:4]=PA_RAM_DN_DLY_SEL [2:0]=PA_RAM_UP_DLY_SEL. Written 0x20 during Init.
	RF_PA_RAMP_DLY = uint8(0x6E)

	// MISC_CFG: ACK pipe, IRQ polarity, PID mode, NDC preamble. (P0)
	// [6]=ENH_NOACK_RX_CONT_DIS [5]=I_NDC_PREAMBLE_SEL [4]=PID_LOW_SEL
	// [3]=IRQ_HIGH_EN [2:0]=ACK_PIPE.
	MISC_CFG = uint8(0x6F)

	// RF_PH_SHFT_H: phase shift K3/K4 high bits. (P0)
	RF_PH_SHFT_H = uint8(0x70)

	// RF_PH_SHFT_K4: phase shift K4 bits [6:0]. (P0)
	RF_PH_SHFT_K4 = uint8(0x71)

	// RF_RESERVED_72: reserved register, do not write. (P0)
	RF_RESERVED_72 = uint8(0x72)

	// RFIRQFLG: interrupt status flags (write 1 to clear). (P0)
	// Use IRQ_* bit constants.
	RFIRQFLG = uint8(0x73)

	// STATUS0: RX pipe number, CI error, and PID (read-only). (P0)
	// [7]=RX_CI_ERR [6:4]=RX_SYNC_ADDR (pipe 0–5; 7=FIFO empty) [3:2]=RX_PID [1:0]=TX_PID.
	STATUS0 = uint8(0x74)

	// STATUS1: received header byte 0 (read-only). (P0)
	// Valid after RX_IRQ. In BLE mode = PDU type byte.
	STATUS1 = uint8(0x75)

	// STATUS2: received header byte 1 (read-only). (P0)
	// In BLE mode = PDU length byte.
	STATUS2 = uint8(0x76)

	// STATUS3: received payload length in bytes (read-only). (P0)
	STATUS3 = uint8(0x77)

	// RF_AGC_STATUS: AGC switch count (read-only). (P0)
	RF_AGC_STATUS = uint8(0x78)

	// RF_AGC_STATUS2: EFUSE IRQ, gain index, AGC iteration count (read-only). (P0)
	RF_AGC_STATUS2 = uint8(0x79)

	// PKT_RSSI_L / PKT_RSSI_H: 14-bit RSSI of the last received packet (read-only). (P0)
	PKT_RSSI_L = uint8(0x7A)
	PKT_RSSI_H = uint8(0x7B)

	// RF_RSSI_ABS_L / _H: 14-bit absolute RSSI value (read-only). (P0)
	RF_RSSI_ABS_L = uint8(0x7C)
	RF_RSSI_ABS_H = uint8(0x7D) // [7]=MAC_IS_IDLE [6]=DEMOD_ACK [5:0]=ABS_VAL[13:8]

	// RT_RSSI_L / RT_RSSI_H: 14-bit real-time ambient noise RSSI (read-only). (P0)
	// On Page 1, 0x7F maps to P1_CAL_STATUS_DONE instead.
	RT_RSSI_L = uint8(0x7E)
	RT_RSSI_H = uint8(0x7F)
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

	// P1_CAL_CTL: calibration FSM control. Dual-use with TXHDR0_CFG (0x1B).
	// [7]=FSM_DCOC [6]=FSM_BW_CAL [5]=FSM_OFST_CAL [4]=FSM_2P_CAL
	// [3]=FSM_VCO_CAL [0]=CAL_EN.
	// Write CAL_* values in sequence. Poll P1_CAL_STATUS_* for completion.
	P1_CAL_CTL = uint8(0x1B)

	// P1_RF_TUNE_27: two-point calibration code offset and cover. (P1)
	// [7]=HIGH_RF [6:5]=CODE_OFFSET [4:0]=TP_CODE_COVER.
	// ES_Tool recommended value: 0xCA (HIGH_RF=1, CODE_OFFSET=4, TP_CODE_COVER=10).
	// Note: SDK example uses 0xAA (CODE_OFFSET=2); ES_Tool and SDK differ on CODE_OFFSET.
	P1_RF_TUNE_27 = uint8(0x27)

	// P1_RF_TUNE_32 / P1_RF_TUNE_33: TX Gaussian filter / FSYN DAC config. (P1)
	// 0x32: [6:5]=INBAND_DELAY [4:0]=GAUSS_SCALE. Init 0x1E.
	// 0x33: [7:6]=OUTBAND_DELAY [5:0]=DF_SEL. Init 0x19.
	P1_RF_TUNE_32 = uint8(0x32)
	P1_RF_TUNE_33 = uint8(0x33)

	// P1_RF_TUNE_37: RX LNA and filter enable. (P1)
	// [7:5]=EN_RXLNA_CG [4]=EN_RXOFSTCMP [3:0]=RXLNA_ICORE. Init 0x15.
	P1_RF_TUNE_37 = uint8(0x37)

	// P1_RF_TUNE_3A: RX filter tuning. (P1)
	// [7:6]=RXFLTRTUNE_OFST [5:4]=RXFLTR_GAIN2B [3]=RXFLTR_GAIN2A [2:0]=RXFLTR_VCM.
	// Bits [7:6] are written during data-rate config: 0=1Mbps/2Mbps, 1=250kbps.
	// Init 0x14.
	P1_RF_TUNE_3A = uint8(0x3A)

	// P1_TX_PWR_AMP: TX PA output current (power level). (P1)
	// [7]=TXPA_POUT_CRNT [4]=TX_DCC_ISEL [3]=LDO_PA_BYPASS_EN [2:0]=TXPA_POUT_RES.
	// 0x17 = 9 dBm; 0x13 = 0 dBm.
	P1_TX_PWR_AMP = uint8(0x3C)

	// P1_RF_TUNE_3E: FSYN PFD and XO hysteresis config. (P1)
	// [4]=EN_FSYNPFD [3:2]=FSYNXO_HYS [1:0]=FSYNPFD_DELAY. Init 0xF1.
	P1_RF_TUNE_3E = uint8(0x3E)

	// P1_RF_TUNE_3F: FSYN charge pump config. (P1)
	// [7]=EN_FSYNCHP [6]=EN_FSYNCHP_SHIFT [4]=EN_FSYNCP_FV [3:0]=FSYNCHP_IOUT.
	// Init 0xD2. For 16 MHz crystal + AGC or 2 Mbps: write 0xD2.
	P1_RF_TUNE_3F = uint8(0x3F)

	// P1_RF_TUNE_40: FSYN LPF config. (P1)
	// [6:4]=FSYNCHP_NSHIFT [3]=FSYNLPF_C3 [2:0]=FSYNCHP_PSHIFT.
	// Init 0x20. For 16 MHz crystal + AGC or 2 Mbps: write 0x20.
	P1_RF_TUNE_40 = uint8(0x40)

	// P1_VCO_PA_CTL: FSYN LPF enable and DAC voltage select. (P1)
	// [7]=EN_FSYNLPF [6]=EN_FSYNLPF_VCDN [5:4]=FSYNLPF_DAC_VSEL
	// [3]=FSYNLPF_BYP_FT [2]=FSYNLPF_R3 [1:0]=FSYNLPF_R1.
	// 0xA6 = 16 MHz crystal; 0xA2 = 32 MHz crystal.
	P1_VCO_PA_CTL = uint8(0x41)

	// P1_CW_TUNE: FSYN VCO enable and tuning. (P1)
	// [7]=EN_FSYNVCO [5:4]=FSYNVCO_VD_SW [3:0]=FSYNVCO_ICORE.
	// 0x4E = carrier-wave active; 0x00 = normal (also 0x05 default).
	P1_CW_TUNE = uint8(0x42)

	// P1_PA_TUNE_43: FSYN VCO bias current. OTP-dependent. (P1)
	// [7:4]=FSYNVCO_IPOLYIPTAT_IBIAS [3]=FSYNVCO_TXCTK [1:0]=FSYNVCO_FC_TUNE.
	// Write 0x10 | calBit (calBit from OTP word 2 bit[4]).
	P1_PA_TUNE_43 = uint8(0x43)

	// P1_PA_BIAS: RF matching network capacitor values. (P1)
	// ES_Tool: EN_RFMTCHNTWKPA + C1VAL/C2VAL TX/RX fields.
	// 0xB0 = 9 dBm matching; 0xBD = 0 dBm matching.
	P1_PA_BIAS = uint8(0x46)

	// P1_PA_TUNE_47: IPOLY current trim. OTP-dependent. (P1)
	// [7]=EN_IPOLY [6:4]=IPOLY_TRIM.
	// Write 0x83 | ((value2 >> 1) & 0x70).
	P1_PA_TUNE_47 = uint8(0x47)

	// P1_TX_PWR_CTL: LDO analog and RFFE trim. (P1)
	// [7:4]=LDO_ANA_TRIM [3:0]=LDO_RFFE_TRIM.
	// 0x88 for both 0 dBm and 9 dBm.
	P1_TX_PWR_CTL = uint8(0x48)

	// P1_TX_DAC: TX DAC configuration. (P1)
	// [7]=TX_DAC_GC (1=high gain for 1Mbps/2Mbps; 0=low gain for 250kbps)
	// [6:4]=TX_DAC_ISEL (always 4) [3]=TX_DAC_CLKINV [2:0]=TX_DAC_VSEL.
	// Written during data-rate configuration.
	P1_TX_DAC = uint8(0x49)

	// P1_RF_TUNE_4C: FSYN XO cap and TX DAC bandwidth. (P1)
	// [7]=FSYNXO_CAP2 [6]=EN_TX_DAC [5]=TX_DAC_BW (1=wide for 2Mbps, 0=narrow)
	// [4]=FSYNXO_STARTUP_FAST [3:0]=LDO_HP_TRIM.
	// Init 0x48. TX_DAC_BW bit written during data-rate config.
	P1_RF_TUNE_4C = uint8(0x4C)

	// P1_CAL_STATUS_PHASE1: RX filter calibration done flag. (P1)
	// [7]=RXFIL_CAL_DONE_3. Poll bit[7] after CAL_PHASE1.
	P1_CAL_STATUS_PHASE1 = uint8(0x6D)

	// P1_CAL_STATUS_VCO: VCO calibration status. (P1)
	// [7]=TWO_POINT_CAL_DONE [6]=VCOAT_MCHCALDONE [5]=VCO_READY [4:0]=TWO_POINT_CODE.
	// Poll bit[6] (VCOAT_MCHCALDONE) after CAL_VCO.
	P1_CAL_STATUS_VCO = uint8(0x70)

	// P1_CAL_STATUS_DONE: frequency / phase-2 calibration done. Dual-use with RT_RSSI_H (0x7F).
	// [7]=REGION_BIN_SRCH_DONE. Poll bit[7] after CAL_FREQ or CAL_PHASE2.
	P1_CAL_STATUS_DONE = uint8(0x7F)
)

// ── STATE_CFG operation codes ─────────────────────────────────────────────────

const (
	// STATE_STB3_INIT: STB3 with EN_LS_3V=0. Written first during Init to reset
	// the state machine before EN_LS_3V is asserted via STATE_STB3.
	STATE_STB3_INIT = uint8(0x04)

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
	IRQ_TX         = uint8(0x80) // TX complete
	IRQ_MAX_RT     = uint8(0x40) // max retransmits reached
	IRQ_ADDR_ERR   = uint8(0x20) // address match error
	IRQ_CRC_ERR    = uint8(0x10) // CRC error
	IRQ_LEN_ERR    = uint8(0x08) // payload length error
	IRQ_PID_ERR    = uint8(0x04) // duplicate PID
	IRQ_RX_TIMEOUT = uint8(0x02) // RX timeout
	IRQ_RX         = uint8(0x01) // valid packet received
	IRQ_ALL        = uint8(0xFF) // all flags
)

// ── SPI_CFG values ────────────────────────────────────────────────────────────

const (
	// SPI_CFG_INIT_3W must be written before entering STB3.
	// REG_SPI3_REN=1 enables 3-wire SPI reads; 0b011 are reserved constant bits.
	// After soft-reset, reading SPI_CFG should return this value (chip-present check).
	SPI_CFG_INIT_3W = uint8(0x83)
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

// ── LP_CFG (0x06) bit constants ───────────────────────────────────────────────
// ES_Tool name for I2C_CFG. Controls LDO, power management, and IRQ muxing.

const (
	LP_CFG_EN_LDO_HP      = uint8(0x80) // enable high-power LDO
	LP_CFG_PWR_UP         = uint8(0x40) // power-up enable
	LP_CFG_EN_PM          = uint8(0x20) // enable power management
	LP_CFG_CE_INT         = uint8(0x10) // CE/interrupt source
	LP_CFG_IRQ_I2C_MUX_EN = uint8(0x08) // mux IRQ signal onto I2C SDA pin (I2C mode: set 1)
	LP_CFG_PTAT_TEMP_TRIM = uint8(0x06) // temperature trim field, bits [2:1]
	LP_CFG_EN_DIG         = uint8(0x01) // enable digital core power
)

// ── WMODE_CFG0 field values ───────────────────────────────────────────────────

const (
	// CRC mode bits [7:6].
	CRC_OFF = uint8(0x00)
	CRC_1B  = uint8(0x40)
	CRC_2B  = uint8(0x80)
	CRC_3B  = uint8(0xC0) // required for BLE

	// CHIP_MODE bit [5] and NORDIC_ENHANCE bit [4].
	// XN297L: both 0. FS01: CHIP_MODE=1, NORDIC_ENHANCE=0. BLE/FS32: both 1.
	CHIP_MODE_BIT      = uint8(0x20) // bit [5]: 1 for FS01 / FS32 / BLE
	NORDIC_ENHANCE_BIT = uint8(0x10) // bit [4]: 1 for FS32 / BLE (not FS01)

	// Composite WORK_MODE values (bits [5:4]) for common modes.
	WORK_MODE_XN297L = uint8(0x00) // XN297L / XN297 compatible
	WORK_MODE_FS01   = uint8(0x20) // FS01 mode
	WORK_MODE_FS32   = uint8(0x30) // FS32 mode
	WORK_MODE_BLE    = uint8(0x30) // BLE mode (same bits as FS32; differentiated by other config)

	// Individual control bits.
	WHITEN_EN_BIT     = uint8(0x08)
	CRC_SKIP_ADDR_BIT = uint8(0x04)
	TX_NOACK_BIT      = uint8(0x02)
	ENDIAN_BIG        = uint8(0x01) // XN297L-compatible
	ENDIAN_LITTLE     = uint8(0x00) // BLE
)

// ── WMODE_CFG1 field values ───────────────────────────────────────────────────

const (
	RX_GOON_BIT     = uint8(0x80) // stay in RX after packet received
	PRI_EXIT_RX_BIT = uint8(0x40) // force exit RX
	FIFO_128_BIT    = uint8(0x20) // 128-byte FIFO (vs 64-byte)
	DPY_EN_BIT      = uint8(0x10) // dynamic payload length
	ENHANCE_BIT     = uint8(0x08) // enhanced mode (auto-ACK, PID)

	// Address width bits [1:0].
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

// ── WHITEN_CFG / SCR_CFG values ──────────────────────────────────────────────

const (
	// WHITEN_SKIP_ADDR_BIT / ACCADDR_SCR_DIS: whitening starts after address field.
	// Not needed in BLE WORK_MODE; address skipping is automatic.
	WHITEN_SKIP_ADDR_BIT = uint8(0x80)
	ACCADDR_SCR_DIS_BIT  = uint8(0x80) // ES_Tool alias

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
	// TX_CFG_MODE bit [7]: 0 = single burst, 1 = continuous carrier.
	TX_SINGLE_BIT     = uint8(0x00)
	TX_CONTINUOUS_BIT = uint8(0x80)

	// RX_CFG_MODE bits [6:5].
	RX_SINGLE_BIT     = uint8(0x00)
	RX_TIMEOUT_BIT    = uint8(0x20) // single with timeout
	RX_CONTINUOUS_BIT = uint8(0x40)

	// PRE_2BYTE_MODE bit [4]: enable 2-byte preamble.
	PRE_2BYTE_MODE_BIT = uint8(0x10)

	// PRE_SYNC_EN bit [0]: preamble detect. Default=1; must remain set.
	PRE_SYNC_EN_BIT = uint8(0x01)

	// TRXMODE_CFG_NORMAL: single TX, continuous RX, pre-sync enabled.
	TRXMODE_CFG_NORMAL = TX_SINGLE_BIT | RX_CONTINUOUS_BIT | PRE_SYNC_EN_BIT // 0x41
)

// ── RF_OSC_CFG (0x37) values ──────────────────────────────────────────────────

const (
	RF_OSC_SEL_BIT     = uint8(0x80) // 1 = external crystal, 0 = internal RC
	ZCONES_OFFSET_MASK = uint8(0x7F) // demodulator zero-crossing offset field

	// RF_OSC_CFG_16MHZ: preset for 16 MHz external crystal.
	// OSC_SEL=1, ZCONES_OFFSET=0x60. Must be written before Page1 OTP read.
	RF_OSC_CFG_16MHZ = uint8(0xE0)

	// RF_OSC_CFG_32MHZ: chip default for 32 MHz crystal.
	// OSC_SEL=0, ZCONES_OFFSET=0x60.
	RF_OSC_CFG_32MHZ = uint8(0x60)
)

// ── RF_DATARATE_CFG field values ──────────────────────────────────────────────
// BW_MODE occupies bits [5:4]. Reserved bits [7:6]=0b01 and [3:0]=0b0101
// must be preserved. The chip resets to 0x45 (1 Mbps); ES_Tool typically
// writes only the BW_MODE field, not the full register.

const (
	// Full register values per ES_Tool (DefaultRegPage0 baseline = 0x45):
	DATARATE_REG_1MBPS   = uint8(0x45) // BW_MODE=0 (1 Mbps) — chip default
	DATARATE_REG_2MBPS   = uint8(0x55) // BW_MODE=1 (2 Mbps)
	DATARATE_REG_250KBPS = uint8(0x75) // BW_MODE=3 (250 kbps)

	// BW_MODE field mask and values (bits [5:4]) for read-modify-write.
	DATARATE_BW_MASK    = uint8(0x30)
	DATARATE_BW_1MBPS   = uint8(0x00) // 1 Mbps
	DATARATE_BW_2MBPS   = uint8(0x10) // 2 Mbps
	DATARATE_BW_250KBPS = uint8(0x30) // 250 kbps

	// Legacy constants kept for compatibility. These names were previously
	// assigned incorrect values (shifted by one rate). Use DATARATE_REG_* instead.
	DATARATE_1MBPS   = uint8(0x45) // corrected: was 0x55 (2 Mbps), now matches ES_Tool
	DATARATE_2MBPS   = uint8(0x55) // corrected: was 0x65, now matches ES_Tool
	DATARATE_250KBPS = uint8(0x75) // unchanged, was already correct
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

// ── RF_PA_MODE_CFG bit constants ──────────────────────────────────────────────
// Written during data-rate configuration and TX power table application.

const (
	// TXPA_MODE_SEL field (bits [5:4]): TX PA operating mode.
	// 0 = mode 0 (250kbps, FS01, FS32); 2 = mode 2 (1Mbps/2Mbps XN297L/BLE).
	RF_PA_MODE_CFG_TXPA_MODE_MASK = uint8(0x30)
	RF_PA_MODE_CFG_TXPA_MODE_0    = uint8(0x00) // 250kbps / FS01 / FS32
	RF_PA_MODE_CFG_TXPA_MODE_2    = uint8(0x20) // 1Mbps / 2Mbps XN297L / BLE

	// FSYNVCO_TXCTK bit [2]: FSYN VCO TX clock.
	// 0 = 1Mbps/250kbps; 1 = 2Mbps (XN297L or BLE).
	RF_PA_MODE_CFG_FSYNVCO_TXCTK = uint8(0x04)

	// RXFLTR_IF field (bits [1:0]): RX filter IF mode.
	// 2 = standard mode for 1Mbps/2Mbps; 3 = 250kbps.
	RF_PA_MODE_CFG_RXFLTR_IF_MASK = uint8(0x03)
	RF_PA_MODE_CFG_RXFLTR_IF_1M   = uint8(0x02)
	RF_PA_MODE_CFG_RXFLTR_IF_250K = uint8(0x03)
)

// ── RF_PA_POUT_CFG bit constants ───────────────────────────────────────────────

const (
	// TXPA_POUT_CRNT field (bits [7:4]): TX PA output current.
	RF_PA_POUT_CFG_POUT_CRNT_MASK = uint8(0xF0)

	// TXPA_LDO_SEL field (bits [3:0]): TX PA LDO voltage select.
	RF_PA_POUT_CFG_LDO_SEL_MASK = uint8(0x0F)
)

// ── RF_AGC_GAIN_CFG (0x4E) bit constants ─────────────────────────────────────

const (
	// GAIN_SEL bit [6]: 1 = use GAIN_OVRD field; 0 = use AGC-computed gain.
	RF_AGC_GAIN_SEL = uint8(0x40)

	// GAIN_OVRD field (bits [5:0]): manual gain override value.
	// Used when GAIN_SEL=1 (non-AGC mode).
	RF_AGC_GAIN_OVRD_MASK    = uint8(0x3F)
	RF_AGC_GAIN_LOW_RX       = uint8(0x2E) // 46: low-gain RX (default for non-AGC)
	RF_AGC_GAIN_HIGH_RX      = uint8(0x3E) // 62: high-gain RX
	RF_AGC_GAIN_CFG_BASELINE = uint8(0x7E) // GAIN_SEL=1 + GAIN_OVRD=62 (recommended initial value)
)

// ── BLEMATCH_CFG0 bit constants ───────────────────────────────────────────────

const (
	SNIF_EN_BIT = uint8(0x80) // sniffer: accept all packets

	// WL_MATCH_MODE bits [6:4]: whitelist filter depth.
	WL_MATCH_NONE = uint8(0x00)
	WL_MATCH_1B   = uint8(0x10) // compare bits [47:40]
	WL_MATCH_2B   = uint8(0x20) // compare bits [47:32]
	WL_MATCH_3B   = uint8(0x30) // compare bits [47:24]
	WL_MATCH_4B   = uint8(0x40) // compare bits [47:16]
	WL_MATCH_5B   = uint8(0x50) // compare bits [47:8]
	WL_MATCH_FULL = uint8(0x60) // compare full 48 bits

	// BLELEN_MATCH_MODE bits [3:2]: length filter.
	BLELEN_DISABLED = uint8(0x00)
	BLELEN_EQUAL    = uint8(0x04)
	BLELEN_GT       = uint8(0x08)
	BLELEN_LT       = uint8(0x0C)
)

// ── MISC_CFG (0x6F) bit constants ────────────────────────────────────────────

const (
	ENH_NOACK_RX_CONT_DIS_BIT = uint8(0x40) // disable continuous RX in enhanced no-ack mode
	NDC_PREAMBLE_SEL_BIT      = uint8(0x20) // select NDC preamble pattern (FS32 mode)
	PID_LOW_SEL_BIT           = uint8(0x10) // PID comparison mode; set in BLE RX
	IRQ_HIGH_EN_BIT           = uint8(0x08) // IRQ pin polarity: 1=active high, 0=active low
	ACK_PIPE_MASK             = uint8(0x07) // ACK pipe selection [2:0]
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
	STATUS0_CI_ERR_BIT = uint8(0x80) // RX coding indicator error
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
	CAL_FREQ    = uint8(0x20) // trigger frequency calibration (chip must be in RX); poll P1_CAL_STATUS_DONE bit[7]
	CAL_PHASE1  = uint8(0x40) // trigger phase calibration 1; poll P1_CAL_STATUS_PHASE1 bit[7]
	CAL_PHASE2  = uint8(0x80) // trigger phase calibration 2; poll P1_CAL_STATUS_DONE bit[7]
	CAL_STOP    = uint8(0x00) // stop all calibration

	CAL_VCO_DONE_BIT    = uint8(0x40) // P1_CAL_STATUS_VCO bit[6]: VCOAT match cal done
	CAL_PHASE1_DONE_BIT = uint8(0x80) // P1_CAL_STATUS_PHASE1 bit[7]: phase 1 done
	CAL_DONE_BIT        = uint8(0x80) // P1_CAL_STATUS_DONE bit[7]: freq / phase 2 done
)

// ── Page 1 OTP constants ──────────────────────────────────────────────────────

const (
	OTP_CTL_START = uint8(0x00) // P1_OTP_CTL: enter OTP read mode
	OTP_CTL_STOP  = uint8(0x01) // P1_OTP_CTL: exit OTP read mode

	OTP_READ_WORD2 = uint8(0x04) // P1_OTP_DATA: command to read word 2 (value2)
	OTP_READ_WORD4 = uint8(0x08) // P1_OTP_DATA: command to read word 4 (value4)

	// OTP word 2 (value2) field masks.
	OTP_VALID_MASK   = uint8(0x0F) // bits [3:0] must equal OTP_VALID_VAL
	OTP_VALID_VAL    = uint8(0x01)
	OTP_CAL_MASK     = uint8(0x10) // bit [4]: CAL_BIT → P1_PA_TUNE_43 bit [0]
	OTP_PA_TRIM_MASK = uint8(0x70) // bits [6:4] after >>1 → P1_PA_TUNE_47 bits [6:4]

	// OTP word 4 (value4) field masks.
	OTP_XTAL_MASK = uint8(0xF0) // bits [7:4]: crystal trim → XTAL_CFG upper nibble
)

// ── Page 1 TX power preset values ─────────────────────────────────────────────
// Apply together via the TX power sequence.

const (
	// P1_TX_PWR_AMP values.
	TX_PWR_AMP_0DBM = uint8(0x13)
	TX_PWR_AMP_9DBM = uint8(0x17)

	// P1_PA_BIAS (matching network) values.
	PA_BIAS_0DBM = uint8(0xBD)
	PA_BIAS_9DBM = uint8(0xB0)

	// P1_TX_PWR_CTL value (same for both power levels).
	TX_PWR_CTL_VAL = uint8(0x88)

	// RF_PA_POUT_CFG values for TX power.
	RF_PA_POUT_CFG_0DBM = uint8(0x84)
	RF_PA_POUT_CFG_9DBM = uint8(0x8C)
)

// ── P1_TX_DAC (0x49) bit constants ───────────────────────────────────────────

const (
	// TX_DAC_GC bit [7]: TX DAC gain select.
	// 1 = high gain (1 Mbps / 2 Mbps); 0 = low gain (250 kbps / FS01 / FS32).
	P1_TX_DAC_GC_BIT = uint8(0x80)

	// TX_DAC_ISEL field (bits [6:4]): TX DAC current select. Always 4.
	P1_TX_DAC_ISEL_MASK = uint8(0x70)
	P1_TX_DAC_ISEL_4    = uint8(0x40) // value 4 shifted into bits [6:4]
)

// ── P1_RF_TUNE_4C (0x4C) bit constants ───────────────────────────────────────

const (
	// TX_DAC_BW bit [5]: TX DAC bandwidth select.
	// 1 = wide bandwidth (2 Mbps); 0 = narrow (1 Mbps / 250 kbps).
	P1_RF_TUNE_4C_TX_DAC_BW = uint8(0x20)
)

// ── P1_RF_TUNE_3A (0x3A) bit constants ───────────────────────────────────────

const (
	// RXFLTRTUNE_OFST field (bits [7:6]): RX filter tuning offset.
	// 0 = 1 Mbps / 2 Mbps; 1 = 250 kbps.
	P1_RF_TUNE_3A_FLTRTUNE_MASK = uint8(0xC0)
	P1_RF_TUNE_3A_FLTRTUNE_1MHZ = uint8(0x00) // 1Mbps and 2Mbps
	P1_RF_TUNE_3A_FLTRTUNE_250K = uint8(0x40) // 250kbps
)

// ── P1_VCO_PA_CTL (0x41) preset values ───────────────────────────────────────

const (
	P1_VCO_PA_CTL_16MHZ = uint8(0xA6) // 16 MHz crystal (EN_FSYNLPF=1, DAC_VSEL, R3/R1)
	P1_VCO_PA_CTL_32MHZ = uint8(0xA2) // 32 MHz crystal
)
