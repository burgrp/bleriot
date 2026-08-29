# BleRiot

BleRiot is a compact request/response protocol for reading and writing named
integer registers on low-power RF nodes. A Linux hub polls the nodes and bridges
their registers to an external [Registry](https://github.com/burgrp/reg)
service. Registry change requests become idempotent absolute register
assignments.

The radio uses 250 kbps GFSK and a BLE-compatible raw packet format. It is not a
standard BLE connection: BleRiot selects raw RF channels and uses its own fixed
packet, addressing, transaction, and encryption rules.

## Architecture

```text
Registry service
       ^  provide values / consume assignments
       |
Linux hub: lib/site
  - inventory and per-node XTEA keys
  - one polling scheduler per RF channel
  - retries, liveness, Registry publication, diagnostics
       |
       | USB HID
       v
MCP2210 --SPI-- PAN211x radio   one passive dongle per active channel
       |
       | 250 kbps BLE-compatible raw RF framing
       v
nodes sharing that channel
```

Each independent group uses one half-duplex RX/TX radio channel. The hub permits
one in-flight `GET` or `SET` transaction on a channel, including its retries and
reply wait. Different channels have independent transaction lanes and run
concurrently.

The [MCP2210 dongle](dongle/mcp2210) is a passive USB-to-SPI bridge driving a
PAN211x. It has no microcontroller, firmware, keys, or protocol state. All
protocol and scheduling logic runs on the Linux host.

## Protocol Summary

Every packet is 13 bytes: a four-byte plaintext source address, a one-byte
plaintext packet version, and one XTEA-encrypted eight-byte block containing
type, flags, register tag, and raw `int32` value. The four packet types are:

| Value | Type | Transaction |
|---:|---|---|
| `0x00` | `GET` | Hub asks for a register; node replies with `VALUE`. |
| `0x01` | `VALUE` | Node returns the current value or `NULL`. |
| `0x02` | `SET` | Hub assigns an absolute value or `NULL`; node replies with `ACK`. |
| `0x03` | `ACK` | Node confirms receipt of a `SET`; resulting state may settle asynchronously. |

There is no transaction token. Channel ownership serializes requests, and the
hub accepts a reply only when its source, register, and response type match the
active transaction. See the [authoritative protocol specification](lib/README.md)
for packet fields, pacing, retry, and register semantics.

## Repository Layout

| Path | Responsibility |
|---|---|
| [lib](lib) | The Go library module and authoritative protocol specification. |
| [lib/shared](lib/shared) | Build-tag-free packages shared by host and TinyGo: packet codec, configuration, inventory, conversions, and chip profiles. |
| [lib/node](lib/node) | Allocation-free firmware runtime for `GET` and `SET`. |
| [lib/site](lib/site) | Linux host engine, polling bridge, diagnostics, CLI, and radio drivers. |
| [example/bob](example/bob) | Reference device type, TinyGo firmware, and inventory-driven host executable. |
| [dongle/mcp2210](dongle/mcp2210) | Passive MCP2210/PAN211x dongle hardware and Linux udev rule. |
| [dongle/py32f403](dongle/py32f403) | Smart-dongle hardware design. |
| [bob](bob) | Reference node PCB. |
| [sub/hw-kicad](sub/hw-kicad) | Shared KiCad symbols and footprints. |

Register identity is a permanent, nonzero per-device-type `uint16` tag.
Deployments are ordinary Go `inventory.Inventory` values; there is no JSON
descriptor. `bleriot make` generates only the TinyGo entry point that bakes one
instance's address, key, channel, spread factor, and config into its firmware.

## Quick Start

Run the reference hub against a Registry service:

```sh
cd example/bob
go run . hub --registry http://localhost:8080 --sweep-interval 1s
```

The hub discovers MCP2210 dongles and assigns them to inventory channels. It
starts with no dongle attached and brings channels online as dongles appear.
Install the shipped udev rule first so the hub can use `/dev/hidraw*` without
root; see [USB access](lib/site/README.md#usb-access).

Useful optional flags include `--timeout 50ms`, `--retries 3`,
`--diagnostics bleriot`, and `--diag-interval 1s`. Put the global `--debug` flag
before the subcommand:

```sh
go run . --debug hub --registry http://localhost:8080 --diagnostics bleriot
```

Create and flash an inventory identity from the reference module:

```sh
cd example/bob
go run . new
go run . make bob flash
```

`new` works offline and prints an `inventory.Instance` stub with a random
nonzero address and XTEA key. `make` selects the inventory instance, generates
its baked firmware entry point, injects the device type's TinyGo and pyocd
targets, and invokes its Makefile.

## Documentation

- [Protocol specification](lib/README.md): authoritative wire format, RF,
  transactions, reliability, register model, and identity.
- [Host library](lib/site/README.md): inventory, commands, polling scheduler,
  Registry behavior, USB access, and diagnostics.
- [Diagnostic metrics](lib/site/DIAGNOSTICS.md): schema 8 catalog, accounting
  rules, and Prometheus examples.

