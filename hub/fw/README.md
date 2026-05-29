# hub/fw — BleRiot "dumb radio modem" firmware

TinyGo firmware that turns the PY32F030 + PAN211x board into a **dumb radio
modem** for the BleRiot hub. It bridges the host's COBS-framed
[link protocol](../link/README.md) (UART) to the PAN211x radio.

The modem holds **no secrets and no protocol intelligence**. It only:

- announces itself to the host on boot with `MsgHello`,
- applies `MsgConfigRadio` (channel + receive address) to the radio,
- transmits `MsgSend` payloads over the air,
- forwards received radio packets to the host as `MsgRecv`,
- reports failures as `MsgError`.

All XTEA encryption, retries, timeouts, and subscription bookkeeping live on the
[host](../host/README.md). See the [protocol spec](../../protocol/README.md).

> Module path: `hub/fw`. Built with TinyGo; not part of the host Go workspace.

---

## Build & flash

All commands run from `hub/fw/`:

```sh
make build         # compile; outputs image.elf + HTML size report
make flash         # build, flash via PyOCD, then attach RTT log
make rtt           # attach RTT log to already-running firmware
make gdb           # start PyOCD GDB server (use arm-none-eabi-gdb elsewhere)
make disassembly   # generate disassembly.txt
make install-pack  # one-time: install PY32F030 CMSIS pack for PyOCD
```

### TinyGo flags of note

- `--gc leaking` — no garbage collection; allocations are permanent. Avoid heap
  allocations in hot paths (the run loop pre-allocates all buffers once).
- `--scheduler tasks` — cooperative scheduling; `runtime.Gosched()` yields
  explicitly.
- `--serial rtt` — `println()` output goes via SEGGER RTT, **independent of the
  UART host link**.
- Target: `py32f030_64k_8k` (64 KB flash, 8 KB RAM — RAM is tight).

---

## Hardware

### Pin assignments

| Signal | Pin | Notes |
|--------|-----|-------|
| UART TX / RX | PB6 / PB7 | USART1, AF0 — the host link |
| SPI SCK | PA9 | → PAN211x pin 2 |
| SPI DATA | PA7 | → PAN211x pin 3, bidirectional (3-wire SPI) |
| SPI CSN | PA10 | → PAN211x pin 1, active-low |
| LED Red | PB0 | |
| LED Green | PB1 | toggles on each received packet |

The radio is driven over a 3-wire SPI interface to the PAN211x; the UART is the
host link.

### Driver layering

```
main.go
  └── pan211x.Driver           (register-level BLE TX/RX logic)
        └── pan211x.Registers  (interface: Read/Write/WriteBuffer/ReadBuffer)
              └── RegistersSPI  (concrete impl over bb/spi master)   ← used here
                  (RegistersI2C is the alternative concrete impl)
```

The `pan211x` driver lives outside this repo
(`github.com/burgrp/tinygo-drivers/pan211x`) and is wired in via `go.work`.
Swapping transports means implementing `pan211x.Registers` — the driver is
unchanged.

---

## PAN211x notes

### Register access protocol

The PAN211x exposes paged registers. The chip has two pages (Page0 = normal
operation, Page1 = calibration). `regPAGE_CFG` (0x00) and `regSTATE_CFG` (0x02)
are accessible from either page. (Over the I2C variant the device address is
0x71 and the first data byte is a register-access byte `reg << 1 | R/W`, not a
plain address; this firmware uses the SPI variant.)

### BLE packet format

The chip auto-inserts the PDU header and length byte when
`PKT_EXT_CFG[HDR_LEN_EXIST]=1`. The TX FIFO must contain **only** AdvA (6 bytes,
LSB-first) + AdvData — no header or length prefix. The auto-inserted header byte
comes from `TXHDR0_CFG`.

BLE whitening seed is bit-reversed in the register:
`WHITEN_CFG = 0x80 | bit_reverse(channel_index | 0x40)`.

### Calibration

`Driver.Init()` must complete a 5-step calibration sequence
(VCO → thermal → frequency → phase×2) before TX works. The thermal step has a
mandatory ~55 ms delay. Frequency calibration requires the chip to be in RX mode
first. Skipping or reordering these steps results in TX timeout.

---

## Run loop

`run()` is a single cooperative loop: drain the UART into the link decoder,
dispatch any complete host commands, then poll the radio for one received packet,
then `runtime.Gosched()`. All buffers are allocated once at boot; the loop never
allocates.

---

## Reference material

PAN211x datasheets, reference manual, and hardware design guide ship with the
external driver under `github.com/burgrp/tinygo-drivers/pan211x` (`doc/`,
`es-tool/`, `examples/`).
