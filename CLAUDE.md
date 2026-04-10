# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Purpose

Mono-repo for the **BleRiot** protocol. Currently contains:
- `bob/` — KiCad PCB design (breakout board v1.3, proven hardware)
- `test-fw/` — TinyGo firmware for the bob board
- `scanner/` — Go BLE scanner utility (runs on a PC/Linux host)
- `sub/hw-kicad/` — shared KiCad symbol/footprint library (git submodule)

## Firmware Build & Flash (test-fw/)

All commands run from `test-fw/`:

```sh
make build    # compile; outputs image.elf + HTML size report
make flash    # build, flash via PyOCD, then attach RTT log
make rtt      # attach RTT log to already-running firmware
make gdb      # start PyOCD GDB server (use arm-none-eabi-gdb in another terminal)
make disassembly  # generate disassembly.txt
make install-pack # one-time: install PY32F030 CMSIS pack for PyOCD
```

TinyGo flags of note (see Makefile):
- `--gc leaking` — no garbage collection; allocations are permanent. Avoid heap allocations in hot paths.
- `--scheduler tasks` — cooperative scheduling; `runtime.Gosched()` yields explicitly.
- `--serial rtt` — `println()` outputs via SEGGER RTT (not UART).
- Target: `py32f030_64k_8k` (64 KB flash, 8 KB RAM — RAM is tight).

## Architecture

### Hardware Pin Assignments
| Signal | Pin |
|--------|-----|
| UART TX/RX | PB6 / PB7 |
| I2C SDA | PA7 (AF12) |
| I2C SCL | PA9 (AF6) |
| LED Red | PB0 |
| LED Green | PB1 |

### Driver Layering

```
main.go
  └── pan211x.Driver          (register-level BLE TX/RX logic)
        └── pan211x.Registers (interface: Read/Write/WriteBuffer/ReadBuffer)
              └── pan211x.RegistersI2C   (concrete impl over i2c.Master)
                    └── i2c.Master       (PY32F030 hardware I2C peripheral)
```

Adding SPI support means implementing `pan211x.Registers` for SPI — the driver is unchanged.

### PAN211x Register Access Protocol

The PAN211x I2C address is **0x71** (7-bit). The bus carries a standard 7-bit device address (`0xE2` write / `0xE3` read), but the *first data byte* is a register access byte formed as `reg << 1 | R/W` — not a plain register address. See `pan211x/i2c.go`.

The chip has two register pages (Page0 = normal operation, Page1 = calibration). `regPAGE_CFG` (0x00) and `regSTATE_CFG` (0x02) are accessible from either page.

### BLE Packet Format

The chip auto-inserts the PDU header and length byte when `PKT_EXT_CFG[HDR_LEN_EXIST]=1` (set to 0x60 in `Init`). The TX FIFO must contain **only** AdvA (6 bytes, LSB-first) + AdvData — no header or length prefix. The auto-inserted header byte comes from `TXHDR0_CFG` (0x42 = `ADV_NONCONN_IND | TxAdd=1`).

BLE whitening seed is bit-reversed in the PAN211x register: `WHITEN_CFG = 0x80 | bit_reverse(channel_index | 0x40)`.

### Calibration

`Driver.Init()` must complete a 5-step calibration sequence (VCO → thermal → frequency → phase×2) before TX works. The thermal step has a mandatory 55 ms delay. Frequency calibration requires the chip to be in RX mode first. Skipping or reordering these steps results in TX timeout.

## Documentation

- PAN211x datasheet, reference manual, hardware design guide: `test-fw/pan211x/doc/`
- PY32F030 datasheet + reference manual: `/home/paul/lib/MCU/Puya/`
- PAN211x SDK examples (SPI-based): `/home/paul/lib/RF/PAN211x/PAN211x-DK-v2.2.5/`
- Saleae Logic 2 HLA for I2C decode: `test-fw/pan211x/saleae/`
