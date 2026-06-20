# BleRiot

BleRiot is a lightweight request/response protocol for reading and writing named
integer registers on low-power RF IoT nodes, bridged to an external
[Registry](https://github.com/burgrp/reg) service. A single **hub** talks to many
small **nodes** over a BLE-compatible 250 kbps radio link; the hub exposes every
node register to the Registry, and turns Registry change requests into radio
writes.

This repository is a multi-module mono-repo containing the protocol definition,
the host hub, an example device firmware and site binary, and the reference
hardware designs.

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
│       site (bleriot hub)    │
│  Linux SBC: protocol logic, │
│  XTEA keys, retries, watch  │
└─────────────────────────────┘
              ▲
              │  USB-HID  (MCP2210 USB-to-SPI bridge)
              ▼
┌─────────────────────────────┐
│        USB radio dongle     │
│   MCP2210 ─SPI─ PAN211x RF  │
│     (no microcontroller)    │
└─────────────────────────────┘
              ▲
              │  RF · BLE-compatible radio link
              ▼
┌─────────────────────────────┐
│          node (×N)          │
└─────────────────────────────┘
```

The hub runs entirely on the Linux host:

- **`site`** (the BleRiot host library) owns all protocol intelligence — per-node
  XTEA keys, register tables, retries/timeouts, push-subscription bookkeeping,
  and the Registry client — and drives the radio directly. A site repository
  drives it with inventory-as-code (see [`example/hub`](example/hub)) and runs it
  on a Linux SBC.
- The **USB radio dongle** is a passive USB-to-SPI bridge: an
  [MCP2210](usb) drives a PAN211x radio over SPI with **no microcontroller and no
  firmware**. The host runs the PAN211x register sequence for every packet over
  USB-HID; the dongle holds no secrets and no protocol state.

The transport is abstracted (`site/radio`): the MCP2210 dongle is one
implementation, and a future smart MCU-resident dongle would slot in unchanged.
One dongle is one radio on one channel; the host fans out across several dongles,
so multiplexing lives in the host, above the wire.

---

## Repository layout

| Path | Module | What it is | Docs |
|------|--------|------------|------|
| [`protocol/`](protocol) | `protocol` | Neutral, dependency-free RF wire format: packet codec + XTEA. Compiles for host and TinyGo alike. | [protocol/README.md](protocol/README.md) — the full **protocol specification** |
| [`site/`](site) | `site` | The BleRiot host library (Linux SBC): protocol engine, USB radio dongle drivers, inventory model, provisioning, Registry bridge. | [site/README.md](site/README.md) |
| [`usb/`](usb) | — | KiCad design for the USB radio dongle (MCP2210 USB-to-SPI bridge + PAN211x). | — |
| [`example/hub/`](example/hub) | `hub` | Example site binary: declares an inventory-as-code deployment and runs the host runtime. | — |
| [`example/thermostat/`](example/thermostat) | `thermostat` | Example device-type module: flat TinyGo firmware (`package main`) plus an importable `spec` subpackage (`Config` + `Type()` + register tags) shared with the host. | — |
| [`bob/`](bob) | — | KiCad PCB design (breakout board v1.3, the reference node hardware). | — |
| [`sub/hw-kicad/`](sub/hw-kicad) | — | Shared KiCad symbol/footprint library (git submodule). | — |

---

## How the pieces fit together

1. **Inventory as code** (see [site](site/README.md)). A deployment is a Go
   program: it declares its devices — each binding a device type's register
   table, the MCU `UID`, XTEA key, channel and config — as an
   `inventory.Inventory` and hands it to `cli.Start`. Register identity on the
   wire is a permanent per-type `Tag`; there is no JSON and no code generation.
   See [protocol §11](protocol/README.md#11-register-model-and-provisioning).

2. **On the node** (firmware, out of scope of this repo's MCU app). The node
   stores raw `int32` per wire ID, encrypts each packet with its XTEA key, and
   answers `GET`/`SET`/`WATCH` over the air. See [protocol §4–§10](protocol/README.md#4-packet-format).

3. **On the dongle** ([usb](usb)). The MCP2210 is a passive USB-to-SPI bridge:
   the host clocks the PAN211x's registers over it to apply channel + receive
   address, transmit packets, and poll for received ones — no secrets, no
   retries, no firmware.

4. **On the host** ([site](site/README.md)). The engine handles XTEA,
   timeouts/retries, and watch refresh; the bridge maps every node register to a
   Registry provider/consumer.

---

## Quick start

### Host hub

```sh
cd example/hub
go run . hub --registry http://localhost:8080 --dongle mcp2210:/dev/hidraw0,37
```

The `--dongle` value is `scheme:selector,channel`: the scheme selects the dongle
type (`mcp2210`), the selector is a `/dev/hidraw*` path or a USB serial, and the
channel is the RF channel. The dongle's `/dev/hidraw*` node is owned by the
`plugdev` group via the shipped udev rule, so no `sudo` is needed (see
[USB access](site/README.md#usb-access)). See [site/README.md](site/README.md)
for the inventory model, commands and flags.

### Provisioning a device

```sh
cd example/hub
go run . new          # read the attached device's UID, print an Instance stub
go run . provision    # write its identity + config to flash over SWD
```

See [site/README.md](site/README.md#provision--new-flags) for the SWD flags.

---

## Module dependencies

```
protocol  ──────────────┬─► site      (also: github.com/burgrp/reg)
 (codec + XTEA)          └─► thermostat firmware (also: pan211x driver, TinyGo)
```

`protocol` is intentionally dependency-free and build-tag-free so the exact same
source compiles into both the Linux host and the TinyGo node firmware,
single-sourcing the on-wire formats.

---

## Documentation index

- **[Protocol specification](protocol/README.md)** — the authoritative wire-format,
  security, transaction, and register-model spec.
- **[Host library](site/README.md)** — inventory-as-code model, the `hub`/`provision`/`new` commands, the USB radio dongle drivers, and internal packages.

