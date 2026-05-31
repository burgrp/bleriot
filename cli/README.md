# cli — BleRiot host library

`cli` is the host (Linux-SBC) half of the BleRiot hub, packaged as a **library**
that a site repository drives with **inventory-as-code**. It owns all protocol
intelligence — per-node XTEA keys, register tables, retries/timeouts, push
subscription bookkeeping, and the Registry client — and drives one or more "dumb
radio modems" ([`hub/fw`](../hub/fw/README.md)) over serial.

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
go run . hub --registry http://localhost:8080 --port /dev/ttyACM0,37
go run . --debug hub --port /dev/ttyACM0,37   # verbose: shows serial traffic
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
| `--baud` | `115200` | Serial baud rate to each modem. |
| `--port` | — | A modem as `device,channel`, e.g. `/dev/ttyACM0,37`. The `,` separator keeps by-path devices (which contain colons) unambiguous, e.g. `/dev/serial/by-path/pci-0000:07:00.4-usb-0:2.1:1.0,37`. Repeatable, one per modem. |

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
| [`pkg/modem`](pkg/modem) | Host-side client for a single modem over one serial port: wraps the [link protocol](../hub/link/README.md). `Port` is a self-healing variant that survives transport loss and reconnects automatically. |
| [`pkg/node`](pkg/node) | Host-side node model: a register descriptor (wire ID → name/type/scaling) built from a device type, plus the provisioned identity (address + key). Bridges values to/from the Registry. |
| [`pkg/bridge`](pkg/bridge) | Connects the engine to the Registry: each register becomes a provider (seeded by `GET`, kept current by `WATCH`), and consumer writes become `SET`. Generic — no per-register knowledge beyond the descriptor. |

---

## Resilience

- The modem `Port` starts even when the radio is **disconnected** and reconnects
  with backoff; `Send` returns `ErrDisconnected` until a transport is available.
- `Watch` subscriptions are persistent intents: they are retained even if the
  initial attempt times out, and re-`WATCH`ed periodically so a node that comes
  up later (or reboots) is resubscribed automatically.
# cli — BleRiot command-line tool

`bleriot` is the BleRiot command-line tool. It is the Linux-SBC half of the hub:
it owns all protocol intelligence — per-node XTEA keys, node descriptors,
retries/timeouts, push subscription bookkeeping, and the Registry client — and
drives one or more "dumb radio modems" ([`hub/fw`](../hub/fw/README.md)) over
serial.

For every BleRiot register the hub acts as a
[Registry](https://github.com/burgrp/reg) provider: it publishes the register's
value and turns consumer change requests into BleRiot `SET` operations.

> Module path: `cli`. See the [protocol spec](../protocol/README.md) for the wire
> format and transaction semantics this implements.

The tool is built with [cobra](https://github.com/spf13/cobra). It has two
subcommands today, `hub` and `generate`; `provision` is planned.

```
bleriot hub        run the host hub bridge (RF nodes ↔ Registry)
bleriot generate   generate node code and the hub descriptor from a spec
bleriot provision  (planned) provision a device's identity
```

---

## Build & run

```sh
make build              # → ./bleriot
make run                # go run ./cmd hub
make test               # go test ./...
make vet
./bleriot hub --config config.json
./bleriot --debug hub   # verbose: shows serial communication
```

A complete, ready-to-edit configuration (config + descriptors + node files) lives
in [`../hub/example`](../hub/example):

```sh
./bleriot hub --config ../hub/example/config.json
```

If `registry` is empty in the config, the `REGISTRY` environment variable is used.

---

## Configuration

The hub is configured with a single JSON file (`--config`, default
`config.json`). Paths in it are resolved relative to the config file's directory.

```json
{
  "registry": "http://localhost:8080",
  "hubAddress": "FFFFFF01",
  "timeoutMs": 50,
  "retries": 3,
  "refreshSeconds": 15,
  "ttlSeconds": 30,
  "baud": 115200,
  "ports": [
    { "device": "/dev/ttyACM0", "channel": 37 }
  ],
  "descriptors": "descriptors",
  "nodes": "nodes"
}
```

| Field | Meaning |
|-------|---------|
| `registry` | Registry service URL (falls back to `$REGISTRY`). |
| `hubAddress` | 4-byte hub source address (hex), used as SRC in outgoing packets. |
| `timeoutMs` | Per-attempt response wait (protocol §9). |
| `retries` | Retransmissions after the first attempt (§9). |
| `refreshSeconds` | How often active `WATCH` subscriptions are refreshed (§10). |
| `ttlSeconds` | Registry provider TTL. |
| `baud` | Serial baud rate to each modem. |
| `ports` | One entry per modem: serial `device` and its radio `channel`. |
| `descriptors` | Directory of generated, content-addressed descriptor files (see below). |
| `nodes` | Directory of per-device node files (see below). |

### Descriptors and node files

The hub does **not** list nodes in the main config. Instead `nodes` names a
directory, and every `*.json` file in it is one physical node. The file's base
name is the node name. Provisioning a new device is a single file drop — the hub
config is never edited.

Descriptors are **content-addressed**: `generate` writes each descriptor as
`<id>.json`, where `<id>` is its descriptor ID (a hash over the
resolved register set). The same ID is embedded in the firmware as
`DescriptorID`, so a provisioned device can report which descriptor it
implements. A node file selects its descriptor by that ID; the hub resolves it
from the `descriptors` pool.

```
config.json               # descriptors: "descriptors", nodes: "nodes"
descriptors/
  1A2B3C4D.json           # shared, generated per-type descriptor (§11.7)
nodes/
  outdoor.json            # node "outdoor"
  garage.json             # node "garage" — same descriptor, own channel + identity
```

```json
// nodes/outdoor.json
{
  "descriptorId": "1A2B3C4D",
  "channel": 37,
  "address": "CCA00002",
  "key": "00112233445566778899AABBCCDDEEFF"
}
```

The `descriptorId` must match an `<id>.json` file in the `descriptors` pool. The
ID is the file name; it is not stored inside the descriptor. See
[protocol §11.9](../protocol/README.md#119-hub-node-files).

---

## Code generation

`bleriot generate` turns a hand-authored JSON spec into the two artifacts
BleRiot needs, from a single run — so firmware and hub can never drift
([protocol §11.7](../protocol/README.md#117-generated-artifacts)):

```sh
./bleriot generate node.json node_gen.go
# arg 1  spec     JSON node spec (input)
# arg 2  fw-go    firmware node code (const wire IDs + RegisterIDs + descriptor ID)
```

The hub descriptor is **not** named on the command line: it is content-addressed
and written next to the spec as `<id>.json`, where `<id>` is the descriptor ID
(also embedded in the firmware). `generate` prints the ID it
wrote. Drop that file into the hub's `descriptors` pool. The only flag is
`--package` (Go package for the generated firmware code, default `main`). Wire
IDs are never authored — they are assigned deterministically by the generator.

The spec is a class library plus a node composed of class instances. Classes,
registers, and instances are keyed by name:

```json
{
  "node": "heating-controller",
  "metadata": { "hw_rev": "1.3" },
  "classes": {
    "thermometer": {
      "registers": {
        "temperature": { "type": "float", "multiplier": 1, "divider": 100 },
        "humidity": { "type": "int", "multiplier": 1, "divider": 1 }
      }
    },
    "switch": { "registers": { "relay": { "type": "bool" } } }
  },
  "instances": {
    "sensor": "thermometer",
    "heating": "switch",
    "led": "switch"
  }
}
```

The register `type` is `int`, `float` (`display = wire × multiplier / divider`),
or `bool`. Non-`bool` registers need a non-zero `divider`. The generator logic
lives in the [`pkg/descriptor`](pkg/descriptor) and [`pkg/codegen`](pkg/codegen)
packages.

---

## Layout

| Path | Responsibility |
|------|----------------|
| [`cmd/`](cmd) | The `bleriot` CLI (cobra): [`main.go`](cmd/main.go) is the root command; [`hub.go`](cmd/hub.go) is the `hub` subcommand (config + node files → modems → engine → bridge); [`generate.go`](cmd/generate.go) is the `generate` subcommand. Colored logging via `tint`. |
| [`pkg/engine`](pkg/engine) | Core protocol logic (§8–§10): XTEA codec per node, `GET`/`SET`/`WATCH`, per-attempt timeout + retransmit, and watch-refresh to keep subscriptions alive within `T_idle`. |
| [`pkg/modem`](pkg/modem) | Host-side client for a single modem over one serial port: wraps the [link protocol](../hub/link/README.md) and exposes configure/send/receive. `Port` is a self-healing variant that survives transport loss and reconnects automatically. |
| [`pkg/node`](pkg/node) | Host-side node model: the generated descriptor (wire ID → name/type/scaling) plus the separately provisioned identity (address + key). Bridges values to/from the Registry. |
| [`pkg/bridge`](pkg/bridge) | Connects the engine to the Registry: each register becomes a provider (seeded by `GET`, kept current by `WATCH`), and consumer writes become `SET`. Generic — no per-register knowledge beyond the descriptor. |
| [`pkg/descriptor`](pkg/descriptor) | Code-generation authoring model (class/register/node spec) and `AllocateIDs` — the deterministic FNV-1a wire-ID allocation (§11.6). |
| [`pkg/codegen`](pkg/codegen) | `GenerateNodeCode` and `GenerateDescriptorJSON` — emit the firmware Go source and hub JSON from a resolved spec (§11.7). |

---

## Resilience

- The modem `Port` starts even when the radio is **disconnected** and reconnects
  with backoff; `Send` returns `ErrDisconnected` until a transport is available.
- `Watch` subscriptions are persistent intents: they are retained even if the
  initial attempt times out, and re-`WATCH`ed periodically so a node that comes
  up later (or reboots) is resubscribed automatically.
