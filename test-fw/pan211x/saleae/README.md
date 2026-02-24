# PAN211x I2C High-Level Analyzer

Saleae Logic 2 High-Level Analyzer extension that decodes register reads and writes
to a **PAN211x 2.4 GHz transceiver** over I2C.

## Setup

1. In Logic 2, add an **I2C** analyzer on the SDA/SCL channels.
2. Open the **Extensions** panel, click the `+` button, choose **Load Existing Extension**,
   and select this directory.
3. Add **PAN211x I2C** as a High-Level Analyzer on top of the I2C analyzer.

## PAN211x I2C Protocol

| Detail | Value |
|---|---|
| Device address (7-bit) | `0x71` |
| Write address byte | `0xE2` |
| Read address byte | `0xE3` |

Every I2C transaction starts with an 8-bit **register access byte** sent by the master
(after the device address byte):

```
bit[7:1]  7-bit register address
bit[0]    R/W direction: 0 = write, 1 = read
```

### Write transaction

```
START | 0xE2 (addr+W) | ACK | reg<<1 (access byte) | ACK | data... | ACK | STOP
```

### Read transaction

```
START | 0xE2 (addr+W) | ACK | reg<<1|1 (access byte) | ACK |
RESTART | 0xE3 (addr+R) | ACK | data... | STOP
```

## Output Frames

Each decoded transaction appears as a single annotated frame:

| Type | Format | Example |
|---|---|---|
| `write` | `W <reg>: [<value>]` | `W STATE_CFG: [STB3]` |
| `read`  | `R <reg>: [<value>]` | `R RFIRQFLG: [0x80 (TX_DONE)]` |
| `error` | `PAN211x Error: <msg>` | `PAN211x Error: unexpected read address at start of transaction` |

## Decoded Registers

| Address | Name | Description | Value decoding |
|---|---|---|---|
| 0x01 | TRX_FIFO | TX/RX FIFO access | raw hex bytes |
| 0x02 | STATE_CFG | Operating mode | SLEEP / STB3 / TX / RX |
| 0x07 | WMODE_CFG0 | CRC, protocol, whitening | field breakdown |
| 0x08 | WMODE_CFG1 | FIFO size, address width | field breakdown |
| 0x09 | RXPLLEN_CFG | RX payload length | raw hex |
| 0x0A | TXPLLEN_CFG | TX payload length | raw hex |
| 0x0F–0x12 | PIPE0_RXADDRx | Pipe0 RX address | raw hex |
| 0x14–0x17 | TXADDRx | TX address | raw hex |
| 0x1A | WHITEN_CFG | Whitening config | skip_addr + seed |
| 0x39 | RF_CHANNEL | RF channel | hex + frequency in MHz |
| 0x73 | RFIRQFLG | Interrupt flags | TX_DONE / RX_DONE |
| 0x77 | STATUS3 | Received payload length | raw hex |

Unknown registers are shown as `REG_0x<addr>`.
