# cli — BleRiot host library

`cli` is the host (Linux-SBC) half of the BleRiot hub, packaged as a **library**
that a site repository drives with **inventory-as-code**. It owns all protocol
intelligence — per-node XTEA keys, register tables, retries/timeouts, push
subscription bookkeeping, and the Registry client — and drives the radio directly
over one or more USB dongles (an [MCP2210](../usb) USB-to-SPI bridge driving a
PAN211x; the dongle has no microcontroller and no firmware).

For every BleRiot register the hub acts as a
[Registry](https://github.com/burgrp/reg) provider: it publishes the register's
value and turns consumer change requests into BleRiot `SET` operations.

> Module path: `cli`. See the [protocol spec](../protocol/README.md) for the wire
> format and transaction semantics this implements.

There is **no JSON configuration and no code generation**. A deployment is a Go
program: it declares an [`inventory.Inventory`](pkg/inventory) and hands it to
[`host.Start`](pkg/host), which builds the `bleriot` command tree (cobra) around
it.

```go
package main

import (
	"cli/pkg/host"
	"cli/pkg/inventory"

	"thermostat"
)

func main() {
	host.Start(inventory.Inventory{
		{
			Name:    "kitchen",
			UID:     [12]byte{ /* MCU unique ID */ },
			Key:     [16]byte{ /* XTEA key */ },
			Channel: 37,
			Type:    thermostat.Type(),
			Config:  thermostat.Config{MinTemp: 18, MaxTemp: 22},
		},
	})
}
```

A complete, runnable site binary lives in [`../example/hub`](../example/hub).

---

## Commands

`host.Start` provides three subcommands:

```
hub        bridge the inventory's RF nodes to the Registry
provision  write a device's identity + config to its flash over SWD
new        read an attached device's UID and print an Instance stub
```

```sh
cd ../example/hub
go run . hub --registry http://localhost:8080 --dongle mcp2210:/dev/hidraw0,37
go run . --debug hub --dongle mcp2210:/dev/hidraw0,37   # verbose: shows radio traffic
go run . provision                            # provision the attached device
go run . new                                  # onboard a brand-new device
```

### `hub` flags

Runtime/deploy settings are command-line flags, not inventory data:

| Flag | Default | Meaning |
|------|---------|---------|
| `--registry` | `$REGISTRY` | Registry service URL. |
| `--hub-address` | `FFFFFF01` | 4-byte hub source address (hex), used as SRC in outgoing packets. |
| `--timeout` | `50ms` | Per-attempt response wait (protocol §9). |
| `--retries` | `3` | Retransmissions after the first attempt (§9). |
| `--refresh` | `15s` | How often active `WATCH` subscriptions are refreshed (§10). |
| `--ttl` | `30s` | Registry provider TTL. |
| `--dongle` | — | A radio dongle as `scheme:selector,channel`, e.g. `mcp2210:/dev/hidraw0,37` or `mcp2210:0001746423,37`. The `scheme` selects the dongle type (only `mcp2210` today — there is no default); the `selector` is a `/dev/hidraw*` path or a USB serial; the channel is split off after the last `,`. Repeatable, one per dongle. `/dev/hidraw*` is root-only, so run under `sudo`. |

### `provision` / `new` flags

Both talk to the attached device over SWD via `pyocd`. The chip-specific details
(pyocd target, UID memory address, provisioning-page flash address) are a
property of the device's MCU and are declared once on the device type's `Chip`
field, so the commands take a single flag:

| Flag | Default | Meaning |
|------|---------|---------|
| `--chip` | — | Chip to drive over SWD. Required only when the inventory declares more than one chip; otherwise the sole chip is selected automatically. |

A `Chip` bundles `Target` (pyocd target name), `UIDAddr` (memory address of the
12-byte MCU unique ID) and `PageAddr` (flash address of the provisioning page).
`inventory.PY32F030` (`py32f030x8`, UID `0x1FFF0E00`, page `0x0800F800`) is
built in; declare a `Chip{...}` on a device type to support other MCUs. `--chip`
accepts a built-in chip name even on an empty inventory, so the very first
device can be onboarded with `new`.

`provision` reads the device's UID, matches it against the inventory **by UID
alone** (no device name argument), and writes its provisioning page — the RF
address (derived as `CRC32(UID)`), key, channel and `Config` — to flash. `new`
reads the UID of a device not yet in the inventory and prints a paste-ready
`inventory.Instance{}` stub to add to the source.

---

## Inventory model

The [`inventory`](pkg/inventory) package is the type-safe model of a deployment.

- **`Register`** — one register of a device type. Its `Tag` (a `uint8`, like a
  protobuf field number) is its permanent wire identity: unique and non-zero
  within the device type, never reused once retired. Slice order is irrelevant.
- **`DeviceType`** — a shared, per-type register table (`Name` + `Registers`),
  authored once in the device-type module and returned by its `Type()` function.
- **`Instance`** — one physical device: its `Name`, MCU `UID`, XTEA `Key`, RF
  `Channel`, device `Type`, and device-specific `Config`.
- **`Inventory`** — the full set of devices. `Validate()` enforces unique
  non-zero tags per device type and unique instance names.

The RF address is never stored: both the host and the firmware derive it as
`CRC32(UID)` ([protocol §11.5](../protocol/README.md)).

### Device-type modules

A device type (e.g. [`../example/thermostat`](../example/thermostat)) is a
dual-target Go module:

- `Config` (a fixed-size struct) is shared by host and firmware.
- `Type() inventory.DeviceType` is host-only (build tag `!tinygo`), so the
  firmware build never pulls in the host library.
- The firmware entry point lives behind `//go:build tinygo`.

### Provisioning page

The host and firmware agree on one flash page per device, encoded by the shared
[`page`](pkg/page) package (`encoding/binary`, fixed-width, CRC-checked):

```
header  magic | layout | configLen | channel | pad | address | key
config  the device type's fixed-size Config struct
crc32   CRC-32 (IEEE) over everything before it
```

The firmware reads it once at boot; `page.IsUnprovisioned` distinguishes an
erased page from a corrupt one.

---

## Layout

| Path | Responsibility |
|------|----------------|
| [`pkg/host`](pkg/host) | The `bleriot` command tree (cobra): `host.Start(Inventory)` plus the `hub`, `provision` and `new` subcommands, and the `Probe` interface (SWD read-UID / write-page) with its `pyocd` implementation. |
| [`pkg/inventory`](pkg/inventory) | The inventory-as-code model: `Register`/`DeviceType`/`Instance`/`Inventory` and `Validate`. |
| [`pkg/page`](pkg/page) | The provisioning page codec, shared verbatim with the firmware (host packs it, firmware reads it). |
| [`pkg/engine`](pkg/engine) | Core protocol logic (§8–§10): XTEA codec per node, `GET`/`SET`/`WATCH`, per-attempt timeout + retransmit, and watch-refresh to keep subscriptions alive within `T_idle`. |
| [`pkg/radio`](pkg/radio) | Transport-agnostic radio adapter: the `Dongle` interface (a single-channel RF endpoint that can `Send`/`Receive`), plus the hub-side `Radio` (receive loop) and node-side `NodeRadio`. The MCP2210 dongle is one `Dongle`; a future smart dongle would be another. |
| [`pkg/radio/mcpdongle`](pkg/radio/mcpdongle) | The `Dongle` implementation over an MCP2210 + PAN211x: brings up the radio, runs the per-packet PAN211x register sequence over USB-HID, and drives the status LEDs. |
| [`pkg/mcp2210`](pkg/mcp2210) | Low-level MCP2210 USB-HID-to-SPI driver (open by `/dev/hidraw*` path or USB serial, chip/GPIO/SPI config, SPI transfers), self-healing against stale/desynced HID responses. |
| [`pkg/node`](pkg/node) | Host-side node model: a register descriptor (wire ID → name/type/scaling) built from a device type, plus the provisioned identity (address + key). Bridges values to/from the Registry. |
| [`pkg/bridge`](pkg/bridge) | Connects the engine to the Registry: each register becomes a provider (seeded by `GET`, kept current by `WATCH`), and consumer writes become `SET`. Generic — no per-register knowledge beyond the descriptor. |

---

## Resilience

- The MCP2210 transport **self-heals** from a desynced HID stream: if an aborted
  session left a stale response queued on the device, `command` discards
  mismatched responses until it reads the one matching its request.
- `Watch` subscriptions are persistent intents: they are retained even if the
  initial attempt times out, and re-`WATCH`ed periodically so a node that comes
  up later (or reboots) is resubscribed automatically.

