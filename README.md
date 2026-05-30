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

- **`cli`** (the `bleriot` tool) owns all protocol intelligence — per-node XTEA
  keys, node descriptors, retries/timeouts, push-subscription bookkeeping, and
  the Registry client. It runs on a Linux SBC as `bleriot hub`.
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
| [`cli/`](cli) | `cli` | The `bleriot` command-line tool (Linux SBC): protocol engine, modem clients, node model, Registry bridge. | [cli/README.md](cli/README.md) |
| [`hub/fw/`](hub/fw) | `hub/fw` | TinyGo "dumb radio modem" firmware for the PY32F030 + PAN211x board. | [hub/fw/README.md](hub/fw/README.md) |
| [`hub/link/`](hub/link) | `hub/link` | Standalone COBS-framed serial link protocol shared by host and firmware. | [hub/link/README.md](hub/link/README.md) |
| [`hub/example/`](hub/example) | — | Ready-to-edit example hub configuration (config + descriptors + node files). | — |
| [`generator/`](generator) | `generator` | Host-side code generator: turns register descriptors into firmware code + hub descriptors. | [generator/README.md](generator/README.md) |
| [`bob/`](bob) | — | KiCad PCB design (breakout board v1.3, the reference hardware). | — |
| [`sub/hw-kicad/`](sub/hw-kicad) | — | Shared KiCad symbol/footprint library (git submodule). | — |

---

## How the pieces fit together

1. **Authoring & generation** ([generator](generator/README.md)). Register
   *classes* and a *node spec* are authored in Go. The generator assigns
   deterministic `uint16` wire IDs and emits two artifacts from one run: firmware
   node code (const IDs) and a hub-side JSON node descriptor. This guarantees
   firmware and hub can never drift. See [protocol §11](protocol/README.md#11-register-model-descriptors-and-code-generation).

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
cd cli
make build          # → ./bleriot
./bleriot hub --config ../hub/example/config.json
```

See [cli/README.md](cli/README.md) for the config format and node files.

### Modem firmware

```sh
cd hub/fw
make flash          # build + flash via PyOCD, then attach RTT log
```

See [hub/fw/README.md](hub/fw/README.md) for hardware pinout and toolchain notes.

### Code generator

```sh
cd generator
go run ./example    # writes generated artifacts to ./example/out
```

See [generator/README.md](generator/README.md).

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
  security, transaction, and code-generation spec.
- **[Command-line tool](cli/README.md)** — `bleriot hub`: configuration, node files, internal packages.
- **[Modem firmware](hub/fw/README.md)** — build/flash, hardware, PAN211x notes.
- **[Link protocol](hub/link/README.md)** — host↔modem COBS framing.
- **[Code generator](generator/README.md)** — authoring model and artifacts.

