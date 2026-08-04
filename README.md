# BleRiot

BleRiot is a lightweight request/response protocol for reading and writing named
integer registers on low-power RF IoT nodes, bridged to an external
[Registry](https://github.com/burgrp/reg) service. A single **hub** talks to many
small **nodes** over a BLE-compatible 250 kbps radio link; the hub exposes every
node register to the Registry, and turns Registry change requests into radio
writes.

This repository is a mono-repo built around a single Go library module,
[`lib`](lib) — which holds the shared wire format, the node firmware runtime, and
the host hub — plus small example modules for the firmware and site binary, and
the reference hardware designs.

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
│      lib/site (bleriot hub) │
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

- **`lib/site`** (the BleRiot host library) owns all protocol intelligence —
  per-node XTEA keys, register tables, retries/timeouts, push-subscription
  bookkeeping, and the Registry client — and drives the radio directly. A site
  repository drives it with inventory-as-code (see [`example/bob`](example/bob))
  and runs it on a Linux SBC.
- The **USB radio dongle** is a passive USB-to-SPI bridge: an
  [MCP2210](usb) drives a PAN211x radio over SPI with **no microcontroller and no
  firmware**. The host runs the PAN211x register sequence for every packet over
  USB-HID; the dongle holds no secrets and no protocol state.

The transport is abstracted (`lib/site/radio`): the MCP2210 dongle is one
implementation, and a future smart MCU-resident dongle would slot in unchanged.
One dongle is one radio on one channel; the host fans out across several dongles,
so multiplexing lives in the host, above the wire.

---

## Repository layout

The code is one Go library module, [`lib`](lib) (`github.com/burgrp/bleriot/lib`),
subdivided into shared, node, and host packages, plus two example modules and the
hardware designs.

| Path | What it is | Docs |
|------|------------|------|
| [`lib/`](lib) | The BleRiot library module. Its top-level README is the full **protocol specification**. | [lib/README.md](lib/README.md) |
| [`lib/shared/`](lib/shared) | Neutral, dependency-free, build-tag-free packages shared by firmware and host: [`protocol`](lib/shared/protocol) (packet codec + XTEA), [`config`](lib/shared/config) (identity primitives and constants), [`inventory`](lib/shared/inventory) (inventory-as-code model). Compile for host and TinyGo alike. | [lib/README.md](lib/README.md) |
| [`lib/node/`](lib/node) | The firmware-side BleRiot runtime: the receive/dispatch loop, XTEA codec and `GET`/`SET`/`WATCH` handling. Imported by node firmware; allocation-free in steady state. | — |
| [`lib/site/`](lib/site) | The BleRiot host library (Linux SBC): protocol engine, USB radio dongle drivers, firmware provisioning generator, Registry bridge. | [lib/site/README.md](lib/site/README.md) |
| [`usb/`](usb) | KiCad design for the USB radio dongle (MCP2210 USB-to-SPI bridge + PAN211x). | — |
| [`example/bob/`](example/bob) | Example device-type module (own module). One flat `package main` holds both targets, split by build tag: the TinyGo node firmware (`//go:build tinygo`) and the example host hub (`//go:build !tinygo`) that declares an inventory-as-code deployment and runs the host runtime. Its importable `spec` subpackage (`Config` + `Type()` + register tags) is shared by both. | — |
| [`bob/`](bob) | KiCad PCB design (breakout board v1.3, the reference node hardware). | — |
| [`sub/hw-kicad/`](sub/hw-kicad) | Shared KiCad symbol/footprint library (git submodule). | — |

---

## How the pieces fit together

1. **Inventory as code** (see [lib/site](lib/site/README.md)). A deployment is a Go
   program: it declares its devices — each binding a device type's register
   table, the MCU `UID`, XTEA key, channel and config — as an
   `inventory.Inventory` and hands it to `cli.Start`. Register identity on the
   wire is a permanent per-type `Tag`; there is no JSON and no code generation.
   See [protocol §11](lib/README.md#11-register-model-and-node-identity).

2. **On the node** (firmware). The node stores raw `int32` per wire ID, encrypts
   each packet with its XTEA key, and answers `GET`/`SET`/`WATCH` over the air
   using the [`lib/node`](lib/node) runtime. See [protocol §4–§10](lib/README.md#4-packet-format).

3. **On the dongle** ([usb](usb)). The MCP2210 is a passive USB-to-SPI bridge:
   the host clocks the PAN211x's registers over it to apply channel + receive
   address, transmit packets, and poll for received ones — no secrets, no
   retries, no firmware.

4. **On the host** ([lib/site](lib/site/README.md)). The engine handles XTEA,
   timeouts/retries, and watch refresh; the bridge maps every node register to a
   Registry provider/consumer.

---

## Quick start

### Host hub

```sh
cd example/bob
go run . hub --registry http://localhost:8080
```

The hub discovers the connected USB radio dongles automatically and assigns them
to the RF channels the inventory uses — no `--dongle` flag. It always starts,
even with no dongle connected: each channel stays offline until a dongle is
available, and a dongle plugged in later is assigned to an orphan channel (and
freed for another when unplugged). A dongle's `/dev/hidraw*` node is owned by the
`plugdev` group via the shipped udev rule, so no `sudo` is needed (see
[USB access](lib/site/README.md#usb-access)). See [lib/site/README.md](lib/site/README.md)
for the inventory model, commands and flags.

### Onboarding a device

```sh
cd example/bob
go run . new          # read the attached device's UID, print an Instance stub
make flash            # bake identity + config into firmware, build, flash over SWD
```

`make build`/`make flash` run `gen` automatically. From a hub that owns the
inventory, `bleriot make <name> flash` does the same for any device: it locates
the firmware source, injects the device's identity and its chip's build/flash
targets, and runs make. See [lib/site/README.md](lib/site/README.md#new-flags)
for the SWD flags.

---

## Module dependencies

```
lib/shared  ──────┬─► lib/site   (also: github.com/burgrp/reg, cobra)
 (codec, config,  ├─► lib/node   (the firmware runtime)
  inventory)      │
                  └─► example/bob ─┬─ firmware  (//go:build tinygo:  lib/node + pan211x, TinyGo)
                                   └─ host hub  (//go:build !tinygo: lib/site)
```

`lib/shared` is intentionally dependency-free and build-tag-free so the exact
same source compiles into both the Linux host and the TinyGo node firmware,
single-sourcing the on-wire formats. The `example/bob` module is dual-target: one
flat `package main` whose firmware (`//go:build tinygo`) and host hub
(`//go:build !tinygo`) entry points are selected by build tag, and it consumes
`lib` via a local `replace` directive.

---

## Documentation index

- **[Protocol specification](lib/README.md)** — the authoritative wire-format,
  security, transaction, and register-model spec.
- **[Host library](lib/site/README.md)** — inventory-as-code model, the `hub`/`gen`/`make`/`new` commands, the USB radio dongle drivers, and internal packages.

