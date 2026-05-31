# BleRiot

BleRiot is a lightweight request/response protocol for reading and writing named
integer registers on low-power RF IoT nodes, bridged to an external
[Registry](https://github.com/burgrp/reg) service. A single **hub** talks to many
small **nodes** over a BLE-compatible 250 kbps radio link; the hub exposes every
node register to the Registry, and turns Registry change requests into radio
writes.

This repository is a multi-module mono-repo containing the protocol definition,
the host hub, the MCU radio-modem firmware, the host-side code generator, and
the reference hardware design.

---

## Architecture at a glance

```
┌─────────────────────────────┐
│        Registry service     │
└─────────────────────────────┘
              ▲
              │  provide / consume
              ▼
┌─────────────────────────────┐
│       cli (bleriot hub)     │
│  Linux SBC: protocol logic, │
│  XTEA keys, retries, watch  │
└─────────────────────────────┘
              ▲
              │  link protocol (COBS / UART)
              ▼
┌─────────────────────────────┐
│            hub/fw           │
│   MCU "dumb radio modem"    │
└─────────────────────────────┘
              ▲
              │  RF · 250 kbps GFSK
              ▼
┌─────────────────────────────┐
│          node (×N)          │
└─────────────────────────────┘
```

The hub is deliberately split in two:

- **`cli`** (the BleRiot host library) owns all protocol intelligence — per-node
  XTEA keys, register tables, retries/timeouts, push-subscription bookkeeping,
  and the Registry client. A site repository drives it with inventory-as-code
  (see [`example/hub`](example/hub)) and runs it on a Linux SBC.
- **`hub/fw`** is a "dumb radio modem": TinyGo firmware that owns only the PAN211x
  radio and holds no secrets. It bridges a COBS-framed serial link to the air.

The two halves speak the **link protocol** (`hub/link`). One modem drives exactly
one radio over one serial port; the host fans out across several modems, so
multiplexing lives in the host, above the wire.

---

## Repository layout

| Path | Module | What it is | Docs |
|------|--------|------------|------|
| [`protocol/`](protocol) | `protocol` | Neutral, dependency-free RF wire format: packet codec + XTEA. Compiles for host and TinyGo alike. | [protocol/README.md](protocol/README.md) — the full **protocol specification** |
| [`cli/`](cli) | `cli` | The BleRiot host library (Linux SBC): protocol engine, modem clients, inventory model, provisioning, Registry bridge. | [cli/README.md](cli/README.md) |
| [`hub/fw/`](hub/fw) | `hub/fw` | TinyGo "dumb radio modem" firmware for the PY32F030 + PAN211x board. | [hub/fw/README.md](hub/fw/README.md) |
| [`hub/link/`](hub/link) | `hub/link` | Standalone COBS-framed serial link protocol shared by host and firmware. | [hub/link/README.md](hub/link/README.md) |
| [`example/hub/`](example/hub) | `hub` | Example site binary: declares an inventory-as-code deployment and runs the host runtime. | — |
| [`example/thermostat/`](example/thermostat) | `thermostat` | Example dual-target device-type module (`Config` + `Type()` + TinyGo firmware). | — |
| [`bob/`](bob) | — | KiCad PCB design (breakout board v1.3, the reference hardware). | — |
| [`sub/hw-kicad/`](sub/hw-kicad) | — | Shared KiCad symbol/footprint library (git submodule). | — |

---

## How the pieces fit together

1. **Inventory as code** (see [cli](cli/README.md)). A deployment is a Go
   program: it declares its devices — each binding a device type's register
   table, the MCU `UID`, XTEA key, channel and config — as an
   `inventory.Inventory` and hands it to `host.Start`. Register identity on the
   wire is a permanent per-type `Tag`; there is no JSON and no code generation.
   See [protocol §11](protocol/README.md#11-register-model-and-provisioning).

2. **On the node** (firmware, out of scope of this repo's MCU app). The node
   stores raw `int32` per wire ID, encrypts each packet with its XTEA key, and
   answers `GET`/`SET`/`WATCH` over the air. See [protocol §4–§10](protocol/README.md#4-packet-format).

3. **On the modem** ([hub/fw](hub/fw/README.md)). The modem applies channel +
   receive address, transmits host packets, and forwards received packets — no
   secrets, no retries.

4. **On the host** ([cli](cli/README.md)). The engine handles XTEA,
   timeouts/retries, and watch refresh; the bridge maps every node register to a
   Registry provider/consumer.

---

## Quick start

### Host hub

```sh
cd example/hub
go run . hub --registry http://localhost:8080 --port /dev/ttyACM0:37
```

See [cli/README.md](cli/README.md) for the inventory model, commands and flags.

### Modem firmware

```sh
cd hub/fw
make flash          # build + flash via PyOCD, then attach RTT log
```

See [hub/fw/README.md](hub/fw/README.md) for hardware pinout and toolchain notes.

### Provisioning a device

```sh
cd example/hub
go run . new          # read the attached device's UID, print an Instance stub
go run . provision    # write its identity + config to flash over SWD
```

See [cli/README.md](cli/README.md#provision--new-flags) for the SWD flags.

---

## Module dependencies

```
protocol  ──────────────┐
 (codec + XTEA)          ├─► cli       (also: hub/link, github.com/burgrp/reg)
                         └─► hub/fw    (also: hub/link, pan211x driver)
hub/link  ──────────────┴─► cli, hub/fw
```

`protocol` and `hub/link` are intentionally dependency-free and build-tag-free so
the exact same source compiles into both the Linux host and the TinyGo firmware,
single-sourcing the on-wire formats.

---

## Documentation index

- **[Protocol specification](protocol/README.md)** — the authoritative wire-format,
  security, transaction, and register-model spec.
- **[Host library](cli/README.md)** — inventory-as-code model, the `hub`/`provision`/`new` commands, and internal packages.
- **[Modem firmware](hub/fw/README.md)** — build/flash, hardware, PAN211x notes.
- **[Link protocol](hub/link/README.md)** — host↔modem COBS framing.

