# PAN211x Register Reference

Sources: official reference manual, PAN211x-DK-v2.2.5 SDK (`pan211.c` and related examples).
Notation: **⚙** = SDK-derived / inferred from code; entries with no qualifier are from the official RM.

---

## Address Space and Page Concept

The chip has a 7-bit register address space (0x00–0x7F, 128 locations). Two independent banks exist behind those addresses:

| Page | Selected by | Contents |
|------|-------------|----------|
| **Page 0** (default) | `PAGE_CFG` = 0x00 | User configuration and status registers |
| **Page 1** | `PAGE_CFG` = 0x01 | Analog RF calibration and TX-power trim |

Two registers are **shared** — they ignore `PAGE_CFG` and are always accessible:
- `PAGE_CFG` (0x00)
- `STATE_CFG` (0x02)

### I2C Access Byte Encoding

The PAN211x I2C address is **0x71** (7-bit). The first data byte in every I2C transaction is a register **access byte**, not a plain register address:

```
access_byte = reg_addr << 1 | R/W   (0=write, 1=read)
```

Write to register 0x07: `START 0xE2 0x0E <data> STOP`
Read from register 0x73: `START 0xE2 0xE6 RESTART 0xE3 <data> STOP`

---

## Page 0 Register Map

| Addr | Name | R/W | Default | Summary |
|------|------|-----|---------|---------|
| 0x00 | PAGE_CFG | R/W | 0x00 | Page bank select (shared) |
| 0x01 | TRX_FIFO | R/W | 0x00 | TX/RX FIFO access point |
| 0x02 | STATE_CFG | R/W | 0x00 | State machine control (shared) |
| 0x03 | SYS_CFG | R/W | 0x02 | System control / soft reset |
| 0x04 | SPI_CFG | R/W | 0x6B | SPI interface config, pull-ups |
| 0x05 | XTAL_CFG | R/W | — | Crystal load-cap trim |
| 0x06 | I2C_CFG | R/W | 0x05 | I2C interface config |
| 0x07 | WMODE_CFG0 | R/W | 0x49 | CRC mode, protocol, whitening, endian |
| 0x08 | WMODE_CFG1 | R/W | 0x83 | FIFO size, DPL, enhanced, address width |
| 0x09 | RXPLLEN_CFG | R/W | 0x00 | Fixed RX payload length |
| 0x0A | TXPLLEN_CFG | R/W | 0x00 | Fixed TX payload length |
| 0x0B | RFIRQ_CFG | R/W | 0x00 | Interrupt mask |
| 0x0C | PID_CFG | R/W | 0x00 | PID manual control, address-error threshold |
| 0x0D | TRXTWTL_CFG | R/W | 0x00 | TX↔RX switch wait time \[7:0\] |
| 0x0E | TRXTWTH_CFG | R/W | 0x00 | TX↔RX switch wait time \[14:8\] |
| 0x0F | PIPE0_RXADDR0 | R/W | 0xCC | Pipe 0 RX address byte 0 (LSB, first on air) |
| 0x10 | PIPE0_RXADDR1 | R/W | 0xCC | Pipe 0 RX address byte 1 |
| 0x11 | PIPE0_RXADDR2 | R/W | 0xCC | Pipe 0 RX address byte 2 |
| 0x12 | PIPE0_RXADDR3 | R/W | 0xCC | Pipe 0 RX address byte 3 |
| 0x13 | PIPE0_RXADDR4 | R/W | 0xCC | Pipe 0 RX address byte 4 (MSB) |
| 0x14 | TXADDR0 | R/W | 0xCC | TX destination address byte 0 (LSB) |
| 0x15 | TXADDR1 | R/W | 0xCC | TX destination address byte 1 |
| 0x16 | TXADDR2 | R/W | 0xCC | TX destination address byte 2 |
| 0x17 | TXADDR3 | R/W | 0xCC | TX destination address byte 3 |
| 0x18 | TXADDR4 | R/W | 0xCC | TX destination address byte 4 (MSB) |
| 0x19 | PKT_EXT_CFG | R/W | 0x00 | Auto-insert header, FEC / spread-spectrum |
| 0x1A | WHITEN_CFG | R/W | 0x7F | Whitening LFSR seed and skip-address flag |
| 0x1B | TXHDR0_CFG | R/W | 0x00 | Auto-inserted TX header byte 0 |
| 0x1C | TXHDR1_CFG | R/W | 0x00 | Auto-inserted TX header byte 1 |
| 0x1D | TXRAMADDR_CFG | R/W | 0x00 | TX FIFO RAM start address |
| 0x1E | RXRAMADDR_CFG | R/W | 0x00 | RX FIFO RAM start address |
| 0x1F | RXPIPE_CFG | R/W | 0x01 | Multi-pipe RX enable |
| 0x20 | PIPE1_RXADDR0 | R/W | 0xCC | Pipe 1 address byte 0 (LSB) |
| 0x21 | PIPE1_RXADDR1 | R/W | 0xCC | Pipe 1 address byte 1 |
| 0x22 | PIPE1_RXADDR2 | R/W | 0xCC | Pipe 1 address byte 2 |
| 0x23 | PIPE1_RXADDR3 | R/W | 0xCC | Pipe 1 address byte 3 |
| 0x24 | PIPE1_RXADDR4 | R/W | 0xCC | Pipe 1 address byte 4 (MSB) |
| 0x25 | PIPE2_RXADDR0 | R/W | 0xCC | Pipe 2 address byte 0 (MSBs shared with Pipe 1) |
| 0x26 | PIPE3_RXADDR0 | R/W | 0xCC | Pipe 3 address byte 0 |
| 0x27 | PIPE4_RXADDR0 | R/W | 0xCC | Pipe 4 address byte 0 |
| 0x28 | PIPE5_RXADDR0 | R/W | 0xCC | Pipe 5 address byte 0 |
| 0x29 | TXAUTO_CFG | R/W | 0x03 | Auto-retransmit delay and count |
| 0x2A | TRXMODE_CFG | R/W | 0x01 | TX/RX mode, pre-sync options |
| 0x2B | RXTIMEOUTL_CFG | R/W | 0xD0 | RX timeout \[7:0\] in µs |
| 0x2C | RXTIMEOUTH_CFG | R/W | 0x07 | RX timeout \[15:8\] in µs |
| 0x2D | BLEMATCH_CFG0 | R/W | 0x00 | BLE sniffer, whitelist filter, length filter |
| 0x2E | BLEMATCH_CFG1 | R/W | 0x28 | Reserved — do not modify |
| 0x2F | WLIST0_CFG | R/W | 0x00 | BLE whitelist AdvA byte 0 |
| 0x30 | WLIST1_CFG | R/W | 0x00 | BLE whitelist AdvA byte 1 |
| 0x31 | WLIST2_CFG | R/W | 0x00 | BLE whitelist AdvA byte 2 |
| 0x32 | WLIST3_CFG | R/W | 0x00 | BLE whitelist AdvA byte 3 |
| 0x33 | WLIST4_CFG | R/W | 0x00 | BLE whitelist AdvA byte 4 |
| 0x34 | WLIST5_CFG | R/W | 0x00 | BLE whitelist AdvA byte 5 |
| 0x35 | BLEMATCHSTART_CFG | R/W | 0x07 | BLE whitelist filter payload start offset |
| 0x36 | RF_DATARATE_CFG | R/W | 0x55 | Air data rate |
| 0x37–0x38 | — | — | — | Undocumented; not written by SDK |
| 0x39 | RF_CHANNEL_CFG | R/W | 0x00 | RF channel: F = 2400 + val \[MHz\] |
| 0x3A–0x42 | ⚙ | W | — | RF analog tuning — undocumented |
| 0x43 | RF_PA_MODE_CFG ⚙ | R/W | 0x32 | PA mode sel \[5:4\], VCO TX clock \[2\], RXFLTR_IF \[1:0\]; data-rate and TX-power dependent |
| 0x44 | RF_PA_POUT_CFG ⚙ | R/W | 0x7C | TX PA output current \[7:4\] and LDO select \[3:0\]; TX-power dependent |
| 0x45 | IRQ_MUX_CFG | R/W | 0x00 | IRQ pin: interrupt / clock-out / PA control |
| 0x46–0x54 | ⚙ | R/W | — | AGC and RSSI control (see `registers.go`) |
| 0x55–0x57 | RF_RSSI_TH1–TH3 ⚙ | W | — | RSSI AGC threshold levels 1–3 |
| 0x58–0x59 | ⚙ | R/W | — | RF analog — undocumented |
| 0x5A–0x5D | RF_RSSI_FIX0–3 ⚙ | W | — | Fixed RSSI calibration words |
| 0x5E–0x61 | RF_GAIN_WORD0–3 ⚙ | W | — | AGC gain table entries; WORD3 changes for high-gain RX |
| 0x62–0x65 | ⚙ | W | — | RF timing — undocumented |
| 0x66 | RF_TX_ANA_TIME ⚙ | W | — | TX analog setup time |
| 0x67 | ⚙ | W | — | RF timing — undocumented |
| 0x68 | RF_RX_PLL_SETUP ⚙ | W | — | RX RF PLL setup time \[5:0\] |
| 0x69–0x6D | ⚙ | W | — | RF timing — undocumented |
| 0x6E | RF_PA_RAMP_DLY ⚙ | W | — | PA ramp delay: DN \[6:4\], UP \[2:0\] |
| 0x6F | MISC_CFG | R/W | 0x00 | ACK pipe number, IRQ polarity |
| 0x70–0x72 | — | — | — | Undocumented |
| 0x73 | RFIRQFLG | R/W | 0x00 | Interrupt flags (write 1 to clear) |
| 0x74 | STATUS0 | R | 0x0C | RX pipe number, PID |
| 0x75 | STATUS1 | R | 0x00 | Received header byte 0 |
| 0x76 | STATUS2 | R | 0x00 | Received header byte 1 |
| 0x77 | STATUS3 | R | 0x00 | Received payload length |
| 0x78–0x79 | — | — | — | Undocumented |
| 0x7A | PKT_RSSI_L | R | 0x00 | Last-packet RSSI \[7:0\] |
| 0x7B | PKT_RSSI_H | R | 0x00 | Last-packet RSSI \[13:8\] |
| 0x7C–0x7D | — | — | — | Undocumented |
| 0x7E | RT_RSSI_L | R | 0x00 | Ambient noise RSSI \[7:0\] |
| 0x7F | RT_RSSI_H | R | 0x00 | Ambient noise RSSI \[13:8\] |

---

## Page 0 Register Details

---

### 0x00 — PAGE_CFG — Page Bank Select
**Shared register — accessible from both pages.**

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:1 | — | — | 0 | Reserved |
| 0 | PAGE_SEL | R/W | 0 | 0 = Page 0 (user registers); 1 = Page 1 (analog RF) |

Always restore to 0x00 after any Page 1 access.

---

### 0x01 — TRX_FIFO — TX/RX FIFO Access

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:0 | FIFO_DATA | R/W | 0x00 | Burst-write: loads TX FIFO. Burst-read: drains RX FIFO. |

The chip maintains separate TX and RX FIFO pointers. In the I2C implementation, each byte is a separate START–STOP transaction; the chip advances its internal pointer on each transaction. The active FIFO (TX or RX) is selected by the current operating mode.

TX: write `len` bytes before asserting TX mode. Data replaces whatever was in the FIFO.
RX: read `len` bytes after `RX_IRQ` fires.

---

### 0x02 — STATE_CFG — State Machine Control
**Shared register — accessible from both pages.**

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7 | TX_FIFO_READY | R/W | 0 | **⚙** Write 1 to trigger TX in continuous TX mode (TX_CFG_MODE=1). Self-clearing. |
| 6 | EN_LS_3V | R/W | 0 | **⚙** High-voltage module enable. |
| 5 | POR_RSTL | R/W | 0 | **⚙** Low-voltage domain reset (active low). |
| 4 | ISO_TO_0 | R/W | 0 | **⚙** Register signal isolation enable. |
| 3 | — | — | 0 | Reserved |
| 2:0 | OPERATE_MODE | R/W | 0 | Operating state — see table below |

**OPERATE_MODE values:**

| Value | State | Description |
|-------|-------|-------------|
| 0 | Deep Sleep | Lowest power; all clocks off; register contents lost |
| 1 | Sleep | Low power; register contents retained |
| 2 | STB1 | Standby 1 |
| 3 | STB2 | Standby 2 |
| 4 | STB3 | Standby 3 — primary idle state; registers accessible |
| 5 | TX | Transmit |
| 6 | RX | Receive (continuous or single, per TRXMODE_CFG) |

**Known write values:**

| Value | Meaning |
|-------|---------|
| 0x04 | Initial STB3 entry (bits 2:0 = 4, high bits = 0) — used at start of Init |
| 0x74 | STB3 with EN_LS_3V=1 (bit 6) — normal idle after init |
| 0x75 | TX mode with EN_LS_3V=1 |
| 0x76 | RX mode with EN_LS_3V=1 |
| 0x21 | Enter sleep (`PAN211_EnterSleep`) |
| 0x22 | Exit sleep / wake-up (`PAN211_ExitSleep`) |

Always write STB3 before reconfiguring registers. The chip must not be in TX or RX when address, payload-length, or mode registers are changed.

---

### 0x03 — SYS_CFG — System Control

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:3 | — | — | — | Reserved — do not modify |
| 2 | IRQ_DATA_MUX_EN | R/W | 0 | Multiplex DATA pin with IRQ output |
| 1 | SOFT_RSTL | R/W | 1 | Logic soft-reset (active low). Write 0 to assert, write 1 to release. |
| 0 | — | — | 0 | Reserved |

**Init sequence:**
`Write 0x00` (assert reset) → 1 ms delay → `Write 0x02` (release reset) → `Write 0x06` (IRQ_DATA_MUX_EN=1 plus release reset).

After soft reset all Page 0 registers return to defaults. SPI_CFG must be re-written.

---

### 0x04 — SPI_CFG — SPI / Bus Interface Config

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7 | REG_SPI3_REN | R/W | 0 | Enable 3-wire SPI read mode. **Must be set before entering STB3.** In 4-wire SPI the DATA line is bidirectional; in 3-wire mode separate MOSI/MISO are used. Not meaningful for I2C operation but the SDK always sets this bit (0x83). |
| 6 | REG_DATA_PUEN | R/W | 1 | DATA/SDA pull-up enable |
| 5 | REG_CSN_PUEN | R/W | 1 | CSN pull-up enable |
| 4 | REG_SCK_PUEN | R/W | 1 | SCK/SCL pull-up enable |
| 3 | REG_IN_PAD_MODE | R/W | 0 | Manual pad configuration mode |
| 2:0 | — | — | 011 | Reserved — forbidden to modify; always include 0b011 |

Default = 0x6B = `REG_DATA_PUEN | REG_CSN_PUEN | REG_SCK_PUEN | 0b011`.

**Page 1 dual use:** When `PAGE_CFG` = 1, address 0x04 maps to the OTP data register (see Page 1 section).

---

### 0x05 — XTAL_CFG — Crystal Load-Capacitor Trim

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:6 | — | ⚙ | 0b11 | **⚙** SDK always ORs 0xC0 into this register — these bits appear to be a required constant or enable field. |
| 5:0 | XTAL_TRIM | R/W | — | Crystal load-capacitor trim value. Loaded from OTP: `(value4 >> 4) | 0xC0` where `value4` is the factory-programmed OTP word. |

Write = `(OTP_value4 >> 4) | 0xC0`. The upper 4 bits of OTP word 4 carry the factory-calibrated trim. Operating with the wrong trim degrades frequency accuracy and may prevent BLE-compatible reception.

**Page 1 dual use:** When `PAGE_CFG` = 1, address 0x05 maps to the OTP mode control register (see Page 1 section).

---

### 0x06 — I2C_CFG — I2C Interface Config

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:4 | — | — | 0 | Reserved |
| 3 | IRQ_I2C_MUX_EN | R/W | 0 | Multiplex IRQ signal onto the I2C SDA pin |
| 2:0 | — | — | 0b101 | Reserved — forbidden to modify |

Default = 0x05 (reserved bits 2:0 = 0b101). SDK does not write this register; the default is sufficient for I2C operation.

---

### 0x07 — WMODE_CFG0 — Work Mode Configuration 0

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:6 | CRC_MODE | R/W | 01 | CRC configuration: `00`=off, `01`=1 byte, `10`=2 bytes, `11`=3 bytes (BLE) |
| 5:4 | WORK_MODE | R/W | 00 | Protocol: `00`=XN297L-compatible, `11`=BLE. Values `01`, `10` not defined. |
| 3 | WHITEN_EN | R/W | 1 | Enable data whitening using LFSR seed from WHITEN_CFG |
| 2 | CRC_SKIP_ADDR | R/W | 0 | Exclude address field from CRC calculation |
| 1 | TX_NOACK | R/W | 0 | Do not wait for ACK in enhanced mode |
| 0 | ENDIAN | R/W | 1 | Bit order: `0`=little-endian (BLE), `1`=big-endian (XN297L-compatible) |

**Mode values used in practice:**

| Value | Binary | CRC | WORK_MODE | Whiten | Endian | Use |
|-------|--------|-----|-----------|--------|--------|-----|
| 0x81 | 10000001 | 2B | XN297L | off | big | XN297L, no whitening |
| 0x89 | 10001001 | 2B | XN297L | on | big | SDK normal TX (`00_normal_tx`) |
| 0xFC | 11111100 | 3B | BLE | on | little | SDK BLE mode |

---

### 0x08 — WMODE_CFG1 — Work Mode Configuration 1

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7 | RX_GOON | R/W | 1 | Stay in RX after packet received (or after CRC/address error). If 0, chip returns to STB3 after each received packet. |
| 6 | PRI_EXIT_RX | R/W | 0 | Force exit from RX immediately (write 1 to abort RX) |
| 5 | FIFO_128_EN | R/W | 0 | `1` = 128-byte FIFO; `0` = 64-byte FIFO |
| 4 | DPY_EN | R/W | 0 | Dynamic payload length: `1` = auto-extract length from received PDU header; `0` = use RXPLLEN_CFG fixed length |
| 3 | ENHANCE | R/W | 0 | Enhanced mode: auto-ACK, PID duplicate detection, max-retransmit |
| 2 | — | — | 0 | Reserved |
| 1:0 | ADDR_BYTE_LEN | R/W | 11 | Address width: `00`=2 B, `01`=3 B, `10`=4 B, `11`=5 B |

**Mode values used in practice:**

| Value | Binary | RX_GOON | FIFO | DPY | ENHANCE | Addr | Use |
|-------|--------|---------|------|-----|---------|------|-----|
| 0xA3 | 10100011 | 1 | 128B | 0 | 0 | 5B | SDK normal TX (`00_normal_tx`) |
| 0xB2 | 10110010 | 1 | 128B | 1 | 0 | 4B | SDK BLE mode |
| 0x9B | 10011011 | 1 | 64B | 1 | 1 | 5B | SDK enhanced mode |
| 0x83 | 10000011 | 1 | 64B | 0 | 0 | 5B | Power-on default |

---

### 0x09 — RXPLLEN_CFG — Receive Payload Length

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:0 | RX_PAYLOAD_LEN | R/W | 0x00 | Fixed receive payload length in bytes. Ignored when `DPY_EN`=1. |

Range: 0–128 (when FIFO_128_EN=1) or 0–64.

---

### 0x0A — TXPLLEN_CFG — Transmit Payload Length

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:0 | TX_PAYLOAD_LEN | R/W | 0x00 | Number of bytes to transmit from FIFO. Must be written before every TX if length varies. |

---

### 0x0B — RFIRQ_CFG — Interrupt Mask

All bits: `0` = interrupt **enabled** (asserts IRQ pin); `1` = interrupt **masked** (suppressed).

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7 | TX_IRQ_MSK | R/W | 0 | Mask TX-complete interrupt |
| 6 | TX_MAX_RT_IRQ_MSK | R/W | 0 | Mask max-retransmit interrupt (enhanced mode) |
| 5 | RX_ADDR_ERR_MSK | R/W | 0 | Mask address-mismatch interrupt (FEC/spread-spectrum mode) |
| 4 | RX_CRC_ERR_IRQ_MSK | R/W | 0 | Mask CRC-error interrupt |
| 3 | RX_LEN_ERR_IRQ_MSK | R/W | 0 | Mask length-error interrupt (enhanced mode) |
| 2 | RX_PID_ERR_IRQ_MSK | R/W | 0 | Mask duplicate-PID interrupt (enhanced mode) |
| 1 | RX_TIMEOUT_IRQ_MSK | R/W | 0 | Mask RX-timeout interrupt |
| 0 | RX_IRQ_MSK | R/W | 0 | Mask RX-complete interrupt |

**Common init values:**

| Value | TX | RX | Errors | Use |
|-------|----|----|--------|-----|
| 0x7F | masked | enabled | masked | SDK normal TX (`00_normal_tx`) |
| 0xEE | enabled | enabled | masked | SDK realtime RSSI (`19_realtime_rssi`) |
| 0x6C | enabled | enabled | partial | SDK sleep example (`09_sleep`) |
| 0x28 | masked | masked | partial | SDK enhanced-mode RX timeout only |

---

### 0x0C — PID_CFG — PID Identifier Configuration

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7 | PID_MANUAL_EN | R/W | 0 | Enable manual PID control (override auto-increment) |
| 6:4 | ADDR_ERR_THR | R/W | 0 | Address-match error threshold (number of bit errors tolerated) |
| 3:2 | RX_PID_MANUAL | R/W | 0 | PID value to expect in received packets (when PID_MANUAL_EN=1) |
| 1:0 | TX_PID_MANUAL | R/W | 0 | PID value to embed in transmitted packets (when PID_MANUAL_EN=1) |

PID (packet ID) is used in enhanced mode to detect duplicate packets. In normal and BLE modes this register is left at 0x00.

---

### 0x0D — TRXTWTL_CFG — TX/RX Switch Wait Time (Low)
### 0x0E — TRXTWTH_CFG — TX/RX Switch Wait Time (High)

| Reg | Bits | Name | R/W | Default | Description |
|-----|------|------|-----|---------|-------------|
| 0x0D | 7:0 | TRX_WAIT\[7:0\] | R/W | 0x00 | Low byte of 15-bit wait-time field |
| 0x0E | 6:0 | TRX_WAIT\[14:8\] | R/W | 0x00 | High 7 bits; bit 7 reserved |

Specifies a wait period (in µs) between TX→RX or RX→TX transitions. Default = 0.

---

### 0x0F–0x13 — PIPE0_RXADDR0–4 — Pipe 0 RX Address

Five-byte hardware address filter for Pipe 0. Only packets whose sync-word matches this address are forwarded to the FIFO and assert `RX_IRQ`.

Byte 0 (0x0F) is the LSB and the first byte sent over the air. Byte 4 (0x13) is the MSB. When using 4-byte addresses (`ADDR_BYTE_LEN`=`10`), byte 4 is ignored; when using 2-byte addresses only bytes 0–1 are used.

Default = 0xCCCCCCCCCC.

The address doubles as the RF sync word — a receiving device only accepts packets where the sender's TXADDR matches its PIPE0_RXADDR.

---

### 0x14–0x18 — TXADDR0–4 — TX Destination Address

Destination address embedded in the packet as the sync word. Must match the intended receiver's `PIPE0_RXADDR` for hardware filtering to pass. Same byte order as PIPE0_RXADDR. Updated before each TX when sending to different destinations.

Default = 0xCCCCCCCCCC.

---

### 0x19 — PKT_EXT_CFG — Packet Extension Configuration

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7 | W_RX_MAX_CTRL_EN | R/W | 0 | **⚙** Enable extended RX length control |
| 6 | HDR_LEN_EXIST | R/W | 0 | Auto-insert header bytes from TXHDR0/1 into every TX packet. When `1`, FIFO must contain **only** payload (no header prefix). |
| 5:4 | HDR_LEN_NUMB | R/W | 00 | Number of header bytes to auto-insert: `00`=0, `01`=1, `10`=2 |
| 3 | PRI_TX_FEC | R/W | 0 | TX spread-spectrum / FEC enable |
| 2 | PRI_RX_FEC | R/W | 0 | RX spread-spectrum / FEC enable |
| 1:0 | PRI_CI_MODE | R/W | 00 | Spread-spectrum mode: `00`=off, `01`=S2, `10`=S8 |

**BLE TX mode:** `PKT_EXT_CFG = 0x60` = `HDR_LEN_EXIST=1, HDR_LEN_NUMB=10` (2 header bytes).
The chip auto-prepends `TXHDR0` (PDU type = 0x42 = `ADV_NONCONN_IND | TxAdd=1`) and `TXHDR1` (auto-calculated PDU length) to the FIFO payload. The FIFO must contain only AdvA (6 bytes, LSB-first) + AdvData.

---

### 0x1A — WHITEN_CFG — Whitening Configuration

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7 | WHITEN_SKIP_ADDR | R/W | 0 | `1` = whitening skips the address field and starts at the payload. Required for BLE (preamble and access address are never whitened in BLE). |
| 6:0 | WHITEN_SEED | R/W | 0x7F | Whitening LFSR initial value. Both TX and RX must use the same seed. |

**Known values:**

| Value | SKIP_ADDR | SEED | Use |
|-------|-----------|------|-----|
| 0x7F | 0 | 0x7F | SDK default / XN297L-compatible |
| 0xD3 | **1** | 0x53 | BLE advertising channel 37 (2402 MHz) |
| 0xB3 | **1** | 0x33 | BLE advertising channel 38 (2426 MHz) |
| 0xF3 | **1** | 0x73 | BLE advertising channel 39 (2480 MHz) |

**BLE whitening seed formula:**

```
WHITEN_CFG = 0x80 | bit_reverse7(BLE_channel_index | 0x40)
```

where `bit_reverse7` reverses all 7 bits and `BLE_channel_index` is the BLE logical channel number (37, 38, or 39 for advertising). **`WHITEN_SKIP_ADDR` (bit 7) must always be set to 1 in BLE mode** — the BLE access address and preamble are never whitened; only the PDU payload is whitened.

Example for BLE ch 37: `37 | 0x40` = `0b1100101`; bit-reversed (7 bits) = `0b1010011` = 0x53; register = `0x80 | 0x53` = **0xD3**.

---

### 0x1B — TXHDR0_CFG — TX Header Byte 0

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:0 | TX_HEADER0 | R/W | 0x00 | Value auto-inserted as the first header byte when `PKT_EXT_CFG.HDR_LEN_EXIST`=1 |

**BLE advertising:** `TXHDR0_CFG = 0x42` = `ADV_NONCONN_IND (PDU type 0x02) | TxAdd=1 (bit 6)`.
(`0b01000010` → bits \[5:4\]=0b10 reserved, bits\[3:0\]=PDU type 0x2=ADV_NONCONN_IND, bit\[6\]=TxAdd=1)

> **Note:** On Page 1, address 0x1B maps to the **Calibration Control** register (different physical register — see Page 1 section).

---

### 0x1C — TXHDR1_CFG — TX Header Byte 1

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:0 | TX_HEADER1 | R/W | 0x00 | Value auto-inserted as the second header byte when `HDR_LEN_NUMB`≥2. In BLE mode the chip fills this with the actual PDU payload length automatically. |

---

### 0x1D — TXRAMADDR_CFG — TX FIFO RAM Start Address

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:0 | TX_RAM_START | R/W | 0x00 | Byte offset into the shared FIFO RAM where the TX payload begins. Normally 0x00. |

---

### 0x1E — RXRAMADDR_CFG — RX FIFO RAM Start Address

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:0 | RX_RAM_START | R/W | 0x00 | Byte offset into the shared FIFO RAM where received data is placed. Normally 0x00. |

---

### 0x1F — RXPIPE_CFG — Multi-Pipe RX Enable

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:6 | — | — | — | Reserved — forbidden |
| 5 | PIPE5_EN | R/W | 0 | Enable RX pipe 5 |
| 4 | PIPE4_EN | R/W | 0 | Enable RX pipe 4 |
| 3 | PIPE3_EN | R/W | 0 | Enable RX pipe 3 |
| 2 | PIPE2_EN | R/W | 0 | Enable RX pipe 2 |
| 1 | PIPE1_EN | R/W | 0 | Enable RX pipe 1 |
| 0 | PIPE0_EN | R/W | 1 | Enable RX pipe 0 (always-on default) |

Write 0x01 to enable Pipe 0 only (typical single-address use). Multi-pipe mode allows simultaneous reception on up to 6 different addresses.

Pipes 2–5 share MSBs with Pipe 1 (only their LSB differs — `PIPE2_RXADDR0`–`PIPE5_RXADDR0`).

---

### 0x20–0x24 — PIPE1_RXADDR0–4 — Pipe 1 RX Address

Full 5-byte address for Pipe 1. Same byte order as PIPE0_RXADDR. Default = 0xCCCCCCCCCC.

---

### 0x25–0x28 — PIPE2–5 RXADDR0 — Pipes 2–5 Address LSB

Pipes 2–5 each have only their LSB (byte 0) independently programmable; bytes 1–4 are copied from Pipe 1's address.

| Reg | Pipe |
|-----|------|
| 0x25 | PIPE2_RXADDR0 |
| 0x26 | PIPE3_RXADDR0 |
| 0x27 | PIPE4_RXADDR0 |
| 0x28 | PIPE5_RXADDR0 |

---

### 0x29 — TXAUTO_CFG — Auto-Retransmit Configuration

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:4 | ARD | R/W | 0000 | Auto-retransmit delay: 250 µs × (ARD + 1). Range: 250 µs–4000 µs. |
| 3:0 | ARC | R/W | 0011 | Auto-retransmit count: number of additional TX attempts (0 = no retransmit, 14 = max). Value 15 is reserved. |

Only used in enhanced mode (`ENHANCE`=1). Normal and BLE modes write 0x00 (no retransmit).

---

### 0x2A — TRXMODE_CFG — TX/RX Mode Configuration

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7 | REG_TX_CFG_MODE | R/W | 0 | TX mode: `0`=single burst (chip returns to STB3 after packet); `1`=continuous carrier |
| 6:5 | REG_RX_CFG_MODE | R/W | 00 | RX mode (normal): `00`=single; `01`=single+timeout; `10`=continuous |
| | | | | RX mode (enhanced): `00`=continuous; `01`=continuous+timeout |
| 4 | PRE_2BYTE_MODE | R/W | 0 | **⚙** Use 2-byte preamble instead of 1-byte |
| 3 | W_PRE_SYNC_12B_EN | R/W | 0 | Enable 12-bit pre-sync pattern |
| 2 | W_PRE_SYNC_8B_EN | R/W | 0 | Enable 8-bit pre-sync pattern |
| 1 | W_PRE_SYNC_4B_EN | R/W | 0 | Enable 4-bit pre-sync pattern |
| 0 | W_PRE_SYNC_EN | R/W | 1 | Enable pre-sync (preamble detect). Default=1; must remain 1 for normal operation. |

**Common values:**

| Value | TX | RX | Use |
|-------|----|----|-----|
| 0x41 | single | continuous | SDK normal mode |
| 0x81 | continuous | — | Carrier-wave test mode |
| 0x61 | single | continuous+timeout | SDK enhanced mode |

---

### 0x2B — RXTIMEOUTL_CFG — RX Timeout (Low)
### 0x2C — RXTIMEOUTH_CFG — RX Timeout (High)

16-bit timeout value in microseconds. RX_TIMEOUT_IRQ fires if no packet is received within this window (requires `REG_RX_CFG_MODE` to include timeout).

| Reg | Bits | Default |
|-----|------|---------|
| 0x2B | \[7:0\] | 0xD0 |
| 0x2C | \[15:8\] | 0x07 |

Default = 0x07D0 = 2000 µs (2 ms). Write low byte first.

---

### 0x2D — BLEMATCH_CFG0 — BLE Filter Configuration

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7 | SNIF_EN | R/W | 0 | Sniffer mode: accept all packets regardless of address |
| 6:4 | WL_MATCH_MODE | R/W | 000 | Whitelist filter depth: `000`=disabled; `001`=compare bits\[47:40\]; `010`=bits\[47:32\]; `011`=bits\[47:24\]; `100`=bits\[47:16\]; `101`=bits\[47:8\]; `110`=full 48 bits |
| 3:2 | BLELEN_MATCH_MODE | R/W | 00 | Length filter: `00`=disabled; `01`=equal; `10`=greater-than; `11`=less-than |
| 1:0 | — | — | 00 | Reserved |

Filter compares the received AdvA field against `WLIST0–5_CFG`. Only used in BLE mode (`WORK_MODE`=11).

SDK BLE TX init: `0x04` (length filter = equal, whitelist disabled).

---

### 0x2E — BLEMATCH_CFG1 — BLE Pattern Match Threshold

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:0 | PATT_MATCH_THRESHOLD | R/W | 0x28 | BLE access address correlation threshold. The chip counts bit matches against `0x8E89BED6`; a packet is accepted only when the count meets this value. Default 0x28 = 40 (of 32 bits) is PANCHIP's empirically tuned setting. Do not modify without PANCHIP guidance. |

---

### 0x2F–0x34 — WLIST0–5_CFG — BLE Whitelist AdvA

Six-byte BLE advertiser address (AdvA) used by the hardware whitelist filter. Byte 0 (0x2F) = bits\[7:0\], byte 5 (0x34) = bits\[47:40\].

Only used in BLE mode with `WL_MATCH_MODE` ≠ 000.

---

### 0x35 — BLEMATCHSTART_CFG — BLE Filter Payload Start

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:6 | — | — | — | Reserved — forbidden |
| 5:0 | PLD_START_BYTE | R/W | 0x07 | Byte offset into the received packet where the whitelist comparison begins. Default = 7. |

SDK BLE TX init writes 0x00 (start from byte 0).

---

### 0x36 — RF_DATARATE_CFG — Air Data Rate

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:6 | — | — | 01 | Reserved constant — always `01` (include in write) |
| 5:4 | DATARATE | R/W | 00 | Air data rate: `00`=1 Mbps, `01`=2 Mbps, `11`=250 kbps |
| 3:0 | — | — | 0101 | Reserved constant — always `0101` (include in write) |

Full register constants (including reserved bits):

| Constant | Value | Rate |
|----------|-------|------|
| DataRate1Mbps | 0x55 | 1 Mbps |
| DataRate2Mbps | 0x65 | 2 Mbps |
| DataRate250kbps | 0x75 | 250 kbps |

BLE requires 1 Mbps.

---

### 0x37–0x38 — Undocumented

Not described in the RM. Purpose inferred from hardware testing.

**0x37 — Crystal pre-configuration gate (16 MHz hardware)**

Must be written to **0xE0** before entering Page 1 for OTP access when using a 16 MHz crystal. Without this write, the Page 1 RF analog tuning values and OTP read do not take effect correctly. Not required (and not written) by the SDK, which targets 32 MHz crystal hardware.

**0x38** — Purpose unknown; not written by SDK or 16 MHz init.

---

### 0x39 — RF_CHANNEL_CFG — RF Channel

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:0 | RF_CH | R/W | 0x00 | Channel number. Center frequency = 2400 + RF_CH \[MHz\]. |

Valid range: 0–83 (2400–2483 MHz, covering the full 2.4 GHz ISM band).

**BLE advertising channels:**

| BLE Channel | RF_CH | Frequency |
|-------------|-------|-----------|
| 37 | 0x02 | 2402 MHz |
| 38 | 0x1A | 2426 MHz |
| 39 | 0x50 | 2480 MHz |

**Calibration channel:** 0x55 = 2485 MHz (outside operating range; used during RF calibration to avoid interference).

---

### 0x3A–0x42 — RF Analog Tuning (Undocumented, Page 0)

⚙ These registers are not documented in the official RM. The SDK writes them during initialization with values derived from the ES_Tool V1.2.6 for a 16 MHz crystal. Do not modify without guidance from PANCHIP.

Not written by the SDK init sequence (the range 0x3A–0x42 is skipped; SDK jumps directly from 0x39 to 0x43).

---

### 0x43 — RF_PA_MODE_CFG (Page 0) ⚙

| Bits | Name | Description |
|------|------|-------------|
| 7 | RXADC_MODE_MANUAL_EN | RX ADC mode manual override enable |
| 6 | RXADC_MODE_SEL | RX ADC mode select |
| 5:4 | TXPA_MODE_SEL | TX PA operating mode: 0=250 kbps/FS01/FS32, 2=1 Mbps/2 Mbps/BLE |
| 3 | EN_RXADCCLK | Enable RX ADC clock |
| 2 | FSYNVCO_TXCTK | FSYN VCO TX clock: 0=1 Mbps/250 kbps, 1=2 Mbps |
| 1:0 | RXFLTR_IF | RX filter IF mode: 2=1 Mbps/2 Mbps, 3=250 kbps |

| SDK Init | TX 0 dBm | TX 9 dBm | Notes |
|----------|----------|----------|-------|
| 0x3A | 0x3A | 0x3A | Same for all power levels |

---

### 0x44 — RF_PA_POUT_CFG (Page 0) ⚙

| Bits | Name | Description |
|------|------|-------------|
| 7:4 | TXPA_POUT_CRNT | TX PA output current (power level select) |
| 3:0 | TXPA_LDO_SEL | TX PA LDO voltage select |

| SDK Init | TX 0 dBm | TX 9 dBm | Notes |
|----------|----------|----------|-------|
| 0x8C | 0x84 | 0x8C | TX-power dependent |

---

### 0x45 — IRQ_MUX_CFG — IRQ Pin Multiplexing

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7:4 | — | — | — | Reserved — forbidden |
| 3:2 | OCLK_SEL | R/W | 00 | Clock frequency when `IRQ_MUX`=01: `00`=1 kHz, `01`=4 kHz, `10`=8 MHz, `11`=16 MHz |
| 1:0 | IRQ_MUX | R/W | 00 | IRQ pin function: `00`=interrupt output, `01`=clock output, `10`=PA control signal |

Not required when IRQ is polled via register read rather than used as a hardware interrupt.

---

### 0x46–0x54 — RF Analog Tuning (Undocumented, Page 0) ⚙

Not written by the SDK in the ES_Tool init sequence for normal operation.

---

### 0x55–0x61 — RSSI Thresholds and AGC Gain Words (Page 0) ⚙

Written during init from the SDK ES_Tool V1.2.6 sequence (16 MHz crystal). Required for correct RF operation.

| Addr | Name | SDK Init | BLE RX mode | Function |
|------|------|----------|-------------|----------|
| 0x55 | RF_RSSI_TH1 | 0xDD | 0xDD | RSSI AGC threshold level 1 |
| 0x56 | RF_RSSI_TH2 | 0xC9 | 0xC9 | RSSI AGC threshold level 2 |
| 0x57 | RF_RSSI_TH3 | 0xB7 | 0xB7 | RSSI AGC threshold level 3 |
| 0x58 | RF_RSSI_INIT | — | — | RSSI initial value for AGC; not written |
| 0x59 | RF_RSSI_HYS | — | — | RSSI hysteresis for AGC; not written |
| 0x5A | RF_RSSI_FIX0 | 0x10 | 0x10 | Fixed RSSI calibration word 0 |
| 0x5B | RF_RSSI_FIX1 | 0xFD | 0xFD | Fixed RSSI calibration word 1 |
| 0x5C | RF_RSSI_FIX2 | 0xE9 | 0xE9 | Fixed RSSI calibration word 2 |
| 0x5D | RF_RSSI_FIX3 | 0xDC | 0xD4 ⚙ | Fixed RSSI calibration word 3; **0xD4** in high-gain RX mode |
| 0x5E | RF_GAIN_WORD0 | 0x02 | 0x02 | AGC gain table entry 0 |
| 0x5F | RF_GAIN_WORD1 | 0x06 | 0x06 | AGC gain table entry 1 |
| 0x60 | RF_GAIN_WORD2 | 0x0E | 0x0E | AGC gain table entry 2 |
| 0x61 | RF_GAIN_WORD3 | 0x2E | 0x3E ⚙ | AGC gain table entry 3; **0x3E** in high-gain RX mode |

---

### 0x62–0x65 — RF Analog Tuning (Undocumented, Page 0) ⚙

Not written by SDK. Purpose unknown.

---

### 0x66 — RF_TX_ANA_TIME (Page 0) ⚙

TX analog setup time. SDK init value: **0x34**.

---

### 0x67 — RF Analog Tuning (Undocumented, Page 0) ⚙

Not written by SDK. Purpose unknown.

---

### 0x68 — RF_RX_PLL_SETUP (Page 0) ⚙

RX RF PLL setup time. Bits \[5:0\] = `RX_RFPLL_SETUP_TIME`. SDK init value: **0x0D**.

---

### 0x69–0x6D — RF Analog Tuning (Undocumented, Page 0) ⚙

Not written by SDK. Purpose unknown.

---

### 0x6E — RF_PA_RAMP_DLY (Page 0) ⚙

PA ramp delay select. Bits \[6:4\] = `PA_RAM_DN_DLY_SEL`, bits \[2:0\] = `PA_RAM_UP_DLY_SEL`. SDK init value: **0x20**. Last register written before Page 1 RF calibration.

---

### 0x6F — MISC_CFG — Miscellaneous Configuration

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7 | — | — | 0 | Reserved |
| 6 | ENH_NOACK_RX_CONT_DIS | R/W | 0 | **⚙** Disable continuous RX after no-ACK in enhanced mode |
| 5 | I_NDC_PREAMBLE_SEL | R/W | 0 | **⚙** Preamble type select |
| 4 | PID_LOW_SEL | R/W | 0 | **⚙** PID comparison mode |
| 3 | IRQ_HIGH_EN | R/W | 0 | IRQ pin polarity: `0`=active low (default), `1`=active high |
| 2:0 | ACK_PIPE | R/W | 0 | Pipe number to use for ACK packets in enhanced mode |

SDK BLE RX example: writes **0x10** to this register — `PID_LOW_SEL`=1.

---

### 0x70–0x72 — Undocumented (Page 0)

Not described in RM. Not written during normal operation. On Page 1, address 0x70 is the VCO calibration status register (see Page 1 section).

---

### 0x73 — RFIRQFLG — RF Interrupt Status Flags

Write `1` to any bit to clear that flag. Flags are set by hardware and cleared by firmware.

| Bits | Name | R/W | Default | Description |
|------|------|-----|---------|-------------|
| 7 | TX_IRQ | R/W | 0 | TX complete: packet sent successfully. In single TX mode, chip has returned to STB3. |
| 6 | TX_MAX_RT_IRQ | R/W | 0 | Max retransmits reached (enhanced mode) — TX failed; chip in STB3 |
| 5 | RX_ADDR_ERR_IRQ | R/W | 0 | Address match error in FEC/spread-spectrum mode |
| 4 | RX_CRC_ERR_IRQ | R/W | 0 | CRC check failed on received packet |
| 3 | RX_LEN_ERR_IRQ | R/W | 0 | Payload length error (enhanced mode) |
| 2 | RX_PID_ERR_IRQ | R/W | 0 | Duplicate PID received (enhanced mode) |
| 1 | RX_TIMEOUT_IRQ | R/W | 0 | RX timeout expired without a packet |
| 0 | RX_IRQ | R/W | 0 | Valid packet received and available in FIFO |

Write **0xFF** to clear all flags. Write **0x80** to clear TX_IRQ only. Write **0x01** to clear RX_IRQ only.

**SDK-defined constants (`pan211.h`):**

| Constant | Value | Flag |
|----------|-------|------|
| RF_IT_TX_IRQ | 0x80 | TX complete |
| RF_IT_MAX_RT_IRQ | 0x40 | Max retransmit |
| RF_IT_ADDR_ERR_IRQ | 0x20 | Address error |
| RF_IT_CRC_ERR_IRQ | 0x10 | CRC error |
| RF_IT_LEN_ERR_IRQ | 0x08 | Length error |
| RF_IT_PID_ERR_IRQ | 0x04 | PID error |
| RF_IT_RX_TIMEOUT_IRQ | 0x02 | RX timeout |
| RF_IT_RX_IRQ | 0x01 | RX complete |
| RF_IT_ALL_IRQ | 0xFF | All flags |


---

### 0x74 — STATUS0 — Status Register 0 (Read-Only)

| Bits | Name | R | Default | Description |
|------|------|---|---------|-------------|
| 7 | RX_CI_ERR | R | 0 | CI error detected in spread-spectrum mode |
| 6:4 | RX_SYNC_ADDR | R | 000 | Pipe number that received the last packet (0–5); 0x7 = FIFO empty |
| 3:2 | RX_PID | R | 11 | PID value of last received packet |
| 1:0 | TX_PID | R | 00 | PID value of last transmitted packet |

Read `RX_SYNC_ADDR` (bits \[6:4\]) to determine which pipe a packet arrived on in multi-pipe mode.

---

### 0x75 — STATUS1 — Received Header Byte 0 (Read-Only)

| Bits | R | Default | Description |
|------|---|---------|-------------|
| 7:0 | R | 0x00 | First received header byte (from TXHDR0 of the sender). Valid after `RX_IRQ`. In BLE mode = PDU type byte (e.g., 0x42 = ADV_NONCONN_IND). |

---

### 0x76 — STATUS2 — Received Header Byte 1 (Read-Only)

| Bits | R | Default | Description |
|------|---|---------|-------------|
| 7:0 | R | 0x00 | Second received header byte (from TXHDR1 of the sender). In BLE mode = PDU length byte. |

---

### 0x77 — STATUS3 — Received Payload Length (Read-Only)

| Bits | R | Default | Description |
|------|---|---------|-------------|
| 7:0 | R | 0x00 | Actual payload byte count stored in FIFO for the last received packet. Read before reading FIFO to know how many bytes to drain. Relevant in dynamic-payload mode. |

---

### 0x78–0x79 — Undocumented (Page 0)

---

### 0x7A — PKT_RSSI_L — Last Packet RSSI (Low)
### 0x7B — PKT_RSSI_H — Last Packet RSSI (High)

| Reg | Bits | R | Description |
|-----|------|---|-------------|
| 0x7A | 7:0 | R | RSSI of last correctly received packet, bits \[7:0\] |
| 0x7B | 5:0 | R | RSSI bits \[13:8\]; bits \[7:6\] reserved |

14-bit RSSI value. Units and reference level not specified in RM. Read after `RX_IRQ` for per-packet signal strength. Larger value = stronger signal.

---

### 0x7C–0x7D — Undocumented (Page 0)

---

### 0x7E — RT_RSSI_L — Ambient Noise RSSI (Low)
### 0x7F — RT_RSSI_H — Ambient Noise RSSI (High)

| Reg | Bits | R | Description |
|-----|------|---|-------------|
| 0x7E | 7:0 | R | Real-time ambient noise RSSI, bits \[7:0\] |
| 0x7F | 5:0 | R | Real-time ambient noise RSSI bits \[13:8\]; bits \[7:6\] reserved |

Continuous background measurement; readable at any time. Compare with `PKT_RSSI` for SNR estimation.

> **Collision with Page 1:** On Page 1, address 0x7F maps to the **calibration-done status** register (see Page 1 section).

---

## Page 1 Register Details

Page 1 is selected by writing `PAGE_CFG` = 0x01. All entries are **⚙ SDK-derived** — none appear in the official RM.

The same address bits 0x00 and 0x02 remain shared (PAGE_CFG, STATE_CFG).

---

### 0x04 (Page 1) — OTP Data Register ⚙

Dual-purpose with Page 0 `SPI_CFG`. On Page 1, used for factory OTP calibration readout.

Write a command byte, then read the result:

| Write | Read result |
|-------|-------------|
| 0x04 | OTP word 2 (value2) |
| 0x08 | OTP word 4 (value4) |

OTP readout requires `0x05` (Page 1) to be set to OTP-read mode first.

---

### 0x05 (Page 1) — OTP Mode Control ⚙

| Value | Meaning |
|-------|---------|
| 0x00 | Enter OTP read mode |
| 0x01 | Exit OTP read mode |

Sequence: `Write 0x05 ← 0x00` → write/read `0x04` → `Write 0x05 ← 0x01`.

---

### OTP Calibration Data Fields ⚙

**OTP word 2 (value2):**

| Bits | Field | Description |
|------|-------|-------------|
| 3:0 | OTP_VALID | Must equal 0x1 to indicate valid OTP. If not, initialization must abort. |
| 4 | CAL_BIT | Used to set bit 0 of Page 1 register 0x43: `calBit = (value2 & 0x10) == 0 ? 1 : 0` |
| 6:5 | PA_TRIM | Used to set bits \[6:4\] of Page 1 register 0x47: `0x83 | ((value2 >> 1) & 0x70)` |
| 7 | — | Unused |

**OTP word 4 (value4):**

| Bits | Field | Description |
|------|-------|-------------|
| 7:4 | XTAL_TRIM | Crystal load-cap trim; written to Page 0 `XTAL_CFG`: `(value4 >> 4) | 0xC0` |
| 3:0 | — | Unused |

---

### 0x1B (Page 1) — Calibration Control Register ⚙

> Shares address with Page 0 `TXHDR0_CFG` — different physical register.

One-hot write to trigger calibration steps. Poll the corresponding status register until done, then write the next step.

| Write | Calibration step | Status register | Done condition | Typical wait |
|-------|-----------------|-----------------|----------------|--------------|
| 0x08 | VCO calibration | 0x70 (Page 1) | bit \[6\] = 1 | < 1 ms |
| 0x10 | Thermal calibration | — | Fixed delay | **55 ms mandatory** |
| 0x20 | Frequency calibration | 0x7F (Page 1) | bit \[7\] = 1 | ~1 ms |
| 0x40 | Phase calibration 1 | 0x6D (Page 1) | bit \[7\] = 1 | ~2 ms |
| 0x80 | Phase calibration 2 | 0x7F (Page 1) | bit \[7\] = 1 | ~2 ms |
| 0x00 | Stop calibration | — | — | — |

Calibration must run in the exact order above. Frequency calibration requires the chip to be in RX mode (`STATE_CFG` = 0x76) before writing 0x20. All other steps can run in STB3.

---

### 0x27 (Page 1) — RF Analog Tuning ⚙

| Init value | TX 0 dBm | TX 9 dBm |
|------------|----------|----------|
| 0xAA | 0xAA | 0xAA |

Written during Page 1 pre-configuration and during TX-power changes. Constant across power levels.

---

### 0x32–0x33 (Page 1) — RF Analog Tuning ⚙

| Addr | Value |
|------|-------|
| 0x32 | 0x1E |
| 0x33 | 0x19 |

Written during Page 1 pre-configuration. Purpose unspecified; do not modify.

---

### 0x37 (Page 1) — RF Analog Tuning ⚙

Init value: **0x15**.

---

### 0x3A (Page 1) — RF Analog Tuning ⚙

Init value: **0x14**.

---

### 0x3C (Page 1) — TX Power Amplitude Control ⚙

Controls PA output level.

| Value | TX power |
|-------|----------|
| 0x13 | 0 dBm |
| 0x17 | 9 dBm |

---

### 0x3E (Page 1) — RF Analog Tuning ⚙

Init value: **0xF1**.

---

### 0x3F (Page 1) — RF Analog Tuning ⚙ (Undocumented)

Not described in the RM or SDK. Required for correct RF operation with a **16 MHz crystal**; init value **0xD2**. Not written by the SDK (which targets 32 MHz hardware).

---

### 0x40 (Page 1) — RF Analog Tuning ⚙ (Undocumented)

Not described in the RM or SDK. Required for correct RF operation with a **16 MHz crystal**; init value **0x20**. Not written by the SDK (which targets 32 MHz hardware).

---

### 0x41 (Page 1) — VCO / PA Control ⚙

| Value | Context |
|-------|---------|
| 0xA6 | Normal operation — **16 MHz crystal** |
| 0xA2 | Normal operation — 32 MHz crystal (SDK default) |
| 0x20 | Carrier-wave test mode entry |
| 0x00 | Carrier-wave test mode exit |

---

### 0x42 (Page 1) — Carrier-Wave Tuning ⚙

| Value | Context |
|-------|---------|
| 0x4E | Carrier-wave test mode |
| 0x00 | Normal / exit CW mode |

Only written when entering/exiting carrier-wave test mode.

---

### 0x43 (Page 1) — OTP-Dependent PA Tuning ⚙

Written from OTP value during initialization:

```
Write(0x43, 0x10 | calBit)
```

where `calBit = (OTP_value2 & 0x10) == 0 ? 1 : 0`.

Possible values: **0x10** (calBit=0) or **0x11** (calBit=1).

---

### 0x46 (Page 1) — PA Bias Control ⚙

| Value | TX power |
|-------|----------|
| 0xB0 | 9 dBm (normal) |
| 0xBD | 0 dBm |

---

### 0x47 (Page 1) — OTP-Dependent PA Tuning ⚙

Written from OTP value during initialization:

```
Write(0x47, 0x83 | ((OTP_value2 >> 1) & 0x70))
```

Base value 0x83; bits \[6:4\] are filled from OTP `PA_TRIM` field.

---

### 0x48 (Page 1) — TX Power Control ⚙

| Value | TX power | Notes |
|-------|----------|-------|
| 0x88 | both | Same value for 0 dBm and 9 dBm |

---

### 0x4C (Page 1) — RF Analog Tuning ⚙

Init value: **0x48**.

---

### 0x6D (Page 1) — Calibration Status: Phase 1 Done ⚙

| Bit | Meaning |
|-----|---------|
| \[7\] | `1` = Phase calibration 1 complete |

Poll after writing `CAL_CTL` = 0x40.

---

### 0x70 (Page 1) — Calibration Status: VCO Done ⚙

| Bit | Meaning |
|-----|---------|
| \[6\] | `1` = VCO calibration complete |

Poll after writing `CAL_CTL` = 0x08.

---

### 0x7F (Page 1) — Calibration Status: Frequency / Phase 2 Done ⚙

| Bit | Meaning |
|-----|---------|
| \[7\] | `1` = Frequency or Phase 2 calibration complete |

Poll after writing `CAL_CTL` = 0x20 (frequency) and again after 0x80 (phase 2).

> **Collision with Page 0:** On Page 0, address 0x7F maps to `RT_RSSI_H`. Always restore `PAGE_CFG` = 0x00 before reading RSSI.

---

## Initialization Sequence

Full sequence from SDK `PAN211_Init()` (`pan211.c`):

```
Step 1 — Bus init (Page 0)
  Write PAGE_CFG   ← 0x00
  Write SPI_CFG    ← 0x83   (REG_SPI3_REN=1, required before STB3)

Step 2 — STB3 with soft reset
  Write STATE_CFG  ← 0x04   (enter STB3, bits set for low-power enable)
  delay 1 ms
  Write STATE_CFG  ← 0x74
  delay 1 ms
  Write SYS_CFG    ← 0x00   (assert SOFT_RSTL)
  delay 1 ms
  Write SYS_CFG    ← 0x02   (release SOFT_RSTL)
  Read  SPI_CFG            → must read 0x83 (chip-present check)
  [16 MHz crystal only] Write 0x37 ← 0xE0  (must precede Page 1 entry)

Step 3 — Read factory OTP (Page 1)
  Write PAGE_CFG   ← 0x01
  Write 0x05       ← 0x00   (OTP read mode)
  Write 0x04       ← 0x04 → Read 0x04 = value2
  Write 0x04       ← 0x08 → Read 0x04 = value4
  Write 0x05       ← 0x01   (exit OTP mode)
  Assert (value2 & 0x0F) == 1  (OTP valid check)
  Write 0x47       ← 0x83 | ((value2 >> 1) & 0x70)
  Write 0x43       ← 0x10 | ((value2 & 0x10) == 0 ? 1 : 0)

Step 4 — Page 1 pre-configuration
  Write 0x27 ← 0xAA
  Write 0x32 ← 0x1E
  Write 0x33 ← 0x19
  Write 0x37 ← 0x15
  Write 0x3A ← 0x14
  Write 0x3E ← 0xF1
  [16 MHz crystal only] Write 0x3F ← 0xD2
  [16 MHz crystal only] Write 0x40 ← 0x20
  Write 0x41 ← 0xA6   (16 MHz crystal) / 0xA2 (32 MHz crystal, SDK default)
  Write 0x46 ← 0xB0   (= PA_BIAS_9DBM)
  Write 0x4C ← 0x48

Step 5 — Page 0 RF configuration
  Write PAGE_CFG     ← 0x00
  Write XTAL_CFG     ← (value4 >> 4) | 0xC0
  Write SYS_CFG      ← 0x06
  Write WMODE_CFG0   ← 0x89  (2B CRC, XN297L, whitening on, big-endian)
  Write WMODE_CFG1   ← 0xA3  (RX_GOON, 128B FIFO, 5B addr)
  Write RXPLLEN_CFG  ← PayloadLen
  Write TXPLLEN_CFG  ← PayloadLen
  Write RFIRQ_CFG    ← 0x7E  (TX+RX enabled, errors masked)
  Write TXAUTO_CFG   ← 0x00  (no retransmit)
  Write TRXMODE_CFG  ← 0x41  (single TX, continuous RX)
  Write WHITEN_CFG   ← 0x7F  (seed=0x7F, no BLE skip)
  Write RXPIPE_CFG   ← 0x01  (Pipe 0 only)
  Write PIPE0_RXADDR0–3 ← OwnAddr bytes [0..3]
  Write RF_CHANNEL   ← 0x55  (calibration channel)
  Write RF_DATARATE  ← DataRate
  Write RF_PA_MODE_CFG ← 0x3A, RF_PA_POUT_CFG ← 0x8C   (PA mode + TX 9 dBm)
  Write RF_RSSI_TH1–RF_GAIN_WORD3, RF_TX_ANA_TIME, RF_RX_PLL_SETUP, RF_PA_RAMP_DLY ← (analog tuning table)

Step 6 — RF calibration (Page 1)
  Write PAGE_CFG   ← 0x01
  Write CAL_CTL    ← 0x08       (start VCO cal)
  Poll  0x70 bit[6] == 1        (VCO done)
  Write CAL_CTL    ← 0x10       (start thermal cal)
  delay 55 ms                   (mandatory)
  Write STATE_CFG  ← 0x76       (enter RX for freq cal)
  delay 200 µs
  Write CAL_CTL    ← 0x20       (start freq cal)
  Poll  0x7F bit[7] == 1        (freq done)
  Write CAL_CTL    ← 0x40       (start phase cal 1)
  Poll  0x6D bit[7] == 1        (phase 1 done)
  Write CAL_CTL    ← 0x80       (start phase cal 2)
  Poll  0x7F bit[7] == 1        (phase 2 done)
  Write CAL_CTL    ← 0x00       (stop cal)
  Write STATE_CFG  ← 0x74       (back to STB3)
  Write PAGE_CFG   ← 0x00

Step 7 — Final setup
  Write RF_CHANNEL ← OperatingChannel
  Write RFIRQFLG   ← 0xFF       (clear all interrupts)
  Write STATE_CFG  ← 0x76       (enter RX)
```

---

## TX Operation

```
Write STATE_CFG  ← 0x74        (STB3 — required before FIFO/address access)
Write TXADDR0–3  ← dst[0..3]   (destination address, 4 bytes LE)
Write TXPLLEN_CFG ← len(payload)
Write TRX_FIFO   ← payload[0..n-1]
Write RFIRQFLG   ← 0xFF        (clear stale flags)
Write STATE_CFG  ← 0x75        (enter TX)
Poll RFIRQFLG bit[7] (TX_IRQ)  (max ~5000 iterations)
Write RFIRQFLG   ← 0xFF
Write STATE_CFG  ← 0x74        (STB3)
Write RFIRQFLG   ← 0xFF
Write STATE_CFG  ← 0x76        (re-enter RX)
```

With `TRXMODE_CFG = 0x41` (single TX), the chip automatically returns to STB3 after transmitting.

---

## RX Operation

With `RX_GOON=1` (WMODE_CFG1 bit 7) the chip remains in RX after each received packet:

```
Poll RFIRQFLG bit[0] (RX_IRQ)  — non-blocking check
If set:
  (Optional) Read STATUS3 (0x77) for dynamic payload length
  (Optional) Read STATUS0 (0x74) bits[6:4] for pipe number
  (Optional) Read STATUS1 (0x75) for received header byte 0
  Read TRX_FIFO ← payload (n separate transactions)
  Write RFIRQFLG ← 0x01        (clear RX_IRQ only)
```

---

## TX Power Configuration

All power-setting registers span both pages. Switch to Page 1, write registers, switch back to Page 0, write Page 0 registers.

| Register | Page | 0 dBm | 9 dBm |
|----------|------|-------|-------|
| P1_RF_TUNE_27 | 1 | 0xAA | 0xAA |
| P1_TX_PWR_AMP | 1 | 0x13 | 0x17 |
| P1_PA_BIAS | 1 | 0xBD | 0xB0 |
| P1_TX_PWR_CTL | 1 | 0x88 | 0x88 |
| RF_PA_MODE_CFG | 0 | 0x3A | 0x3A |
| RF_PA_POUT_CFG | 0 | RF_PA_POUT_CFG_0DBM (0x84) | RF_PA_POUT_CFG_9DBM (0x8C) |

---

## Carrier-Wave Test Mode

Used for regulatory testing (continuous unmodulated carrier).

```
Write PAGE_CFG  ← 0x01
Write 0x41      ← 0x20
Write 0x42      ← 0x4E
Write PAGE_CFG  ← 0x00
Write TRXMODE_CFG ← 0x81    (continuous TX)
Write STATE_CFG ← 0x75      (TX)
```

Exit:
```
Write PAGE_CFG  ← 0x01
Write 0x41      ← 0x00
Write 0x42      ← 0x00
Write PAGE_CFG  ← 0x00
Write STATE_CFG ← 0x74      (STB3)
Write TRXMODE_CFG ← 0x41    (restore single TX)
```

---

## Sleep / Wakeup (`09_sleep` example)

```
Enter sleep:
  Write STATE_CFG ← 0x74   (STB3)
  Write STATE_CFG ← 0x21   (Sleep)

Exit sleep:
  Write STATE_CFG ← 0x22   (Wake)
  Write STATE_CFG ← 0x74   (STB3)
  delay 1 ms                (crystal stabilisation)
  [resume normal TX/RX]
```

Register state is preserved during sleep. The crystal oscillator stops; the 1 ms delay after wakeup allows it to restabilise before RF operation.

---

## BLE Mode Configuration Differences

When `WORK_MODE = 11` (BLE), the following registers differ from XN297L-compatible mode:

| Register | XN297L | BLE | Notes |
|----------|--------|-----|-------|
| WMODE_CFG0 | 0x89 | 0xFC | 3B CRC, BLE mode, whitening, little-endian |
| WMODE_CFG1 | 0xA2 | 0xB2 | DPY_EN=1 (auto-length from PDU) |
| WHITEN_CFG | 0x7F | 0xD3 / 0xB3 / 0xF3 | BLE channel seed (ch 37/38/39); SKIP_ADDR=1 required in BLE mode |
| PKT_EXT_CFG | 0x00 | 0x60 | Auto-insert 2 header bytes |
| TXHDR0_CFG | 0x00 | 0x42 | ADV_NONCONN_IND \| TxAdd=1 |
| BLEMATCH_CFG0 | 0x00 | 0x04 | Length filter = equal |
| BLEMATCHSTART_CFG | 0x07 | 0x00 | Filter from byte 0 |
| RF_DATARATE_CFG | any | 0x55 | BLE requires 1 Mbps |
| RF_RSSI_FIX3 ⚙ | 0xDC | 0xD4 | RSSI calibration word 3 |
| RF_GAIN_WORD3 ⚙ | 0x2E | 0x3E | AGC gain table entry 3 |
| 0x6F (Page 0) ⚙ | 0x00 | 0x10 | PID_LOW_SEL=1 in BLE RX |

In BLE TX: FIFO payload = AdvA (6 bytes LSB-first) + AdvData only. Header (PDU type + length) auto-inserted by chip.
