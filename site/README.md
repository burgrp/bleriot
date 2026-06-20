# site — BleRiot host library

`site` is the host (Linux-SBC) half of the BleRiot hub, packaged as a **library**
that a site repository drives with **inventory-as-code**. It owns all protocol
intelligence — per-node XTEA keys, register tables, retries/timeouts, push
subscription bookkeeping, and the Registry client — and drives the radio directly
over one or more USB dongles (an [MCP2210](../usb) USB-to-SPI bridge driving a
PAN211x; the dongle has no microcontroller and no firmware).

For every BleRiot register the hub acts as a
[Registry](https://github.com/burgrp/reg) provider: it publishes the register's
value and turns consumer change requests into BleRiot `SET` operations.

> Module path: `site`. See the [protocol spec](../protocol/README.md) for the wire
> format and transaction semantics this implements.

There is **no JSON configuration and no code generation**. A deployment is a Go
program: it declares an [`inventory.Inventory`](inventory) and hands it to
[`cli.Start`](cli), which builds the `bleriot` command tree (cobra) around
it.

```go
package main

import (
	"site/cli"
	"site/inventory"

	"thermostat"
)

func main() {
	cli.Start(inventory.Inventory{
		{
			Name:    "kitchen",
			UID:     [12]byte{ /* MCU unique ID */ },
			Key:     [16]byte{ /* XTEA key */ },
			Channel: inventory.Channel{Name: "far", Number: 37},
			Type:    thermostat.Type(),
			Config:  thermostat.Config{MinTemp: 18, MaxTemp: 22},
		},
	})
}
```

A complete, runnable site binary lives in [`../example/hub`](../example/hub).

---

## Commands

`cli.Start` provides three subcommands:

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
| `--dongle` | — | A radio dongle as `scheme:selector,channel`, e.g. `mcp2210:/dev/hidraw0,37` or `mcp2210:0001746423,37`. The `scheme` selects the dongle type (only `mcp2210` today — there is no default); the `selector` is a `/dev/hidraw*` path or a USB serial; the channel is split off after the last `,`. Repeatable, one per dongle. The dongle's `/dev/hidraw*` node is root-only by default; install the udev rule (see [USB access](#usb-access)) to use it without `sudo`. |
| `--diagnostics` | — (off) | Publish hub-synthesised diagnostic registers under this Registry namespace prefix (e.g. `diag`). Empty disables them. See [Diagnostics](#diagnostics). |
| `--diag-window` | `30s` | Averaging window for the diagnostic `rate.*` registers. |

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

### USB access

The MCP2210 dongle appears as a `/dev/hidraw*` node that is root-only by
default, so the `hub` would otherwise need `sudo`. Install the udev rule shipped
with the dongle hardware design — [`../usb/99-bleriot-mcp2210.rules`](../usb/99-bleriot-mcp2210.rules) —
to grant access to the local desktop user and the `plugdev` group instead:

```sh
sudo cp ../usb/99-bleriot-mcp2210.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
sudo udevadm trigger --subsystem-match=hidraw --action=change
```

Re-plug the dongle if it was already connected, and make sure your user is in
the `plugdev` group (`id` should list it; otherwise `sudo usermod -aG plugdev
$USER` and re-login). The rule matches the MCP2210 by its USB ID `04d8:00de`.

> Provisioning over SWD (`provision` / `new`) drives the SWD probe through
> `pyocd`, which has its own access requirements independent of this rule.

---

## Inventory model

The [`inventory`](inventory) package is the type-safe model of a deployment.

- **`Register`** — one register of a device type. Its `Tag` (a `uint8`, like a
  protobuf field number) is its permanent wire identity: unique and non-zero
  within the device type, never reused once retired. Slice order is irrelevant.
- **`DeviceType`** — a shared, per-type register table (`Name` + `Registers`),
  authored once in the device-type module and returned by its `Type()` function.
- **`Instance`** — one physical device: its `Name`, MCU `UID`, XTEA `Key`, RF
  `Channel`, device `Type`, and device-specific `Config`.
- **`Channel`** — an RF channel `Number` together with the `SpreadFactor` every
  node on it uses. Spreading factor is a property of the channel (a dongle
  transmits one factor at a time), so binding the two makes it impossible to give
  two nodes on one channel different factors. Declare each channel once and share
  that value across instances; the zero `SpreadFactor` is the highest-range S8.
  Each channel also has a required, unique `Name` (e.g. `"far"`) that scopes its
  per-dongle [diagnostic registers](#diagnostics).
- **`Inventory`** — the full set of devices. `Validate()` enforces unique
  non-zero tags per device type, unique instance names, one spreading factor per
  channel, and a non-empty, one-to-one mapping between channel numbers and names.

The RF address is never stored: both the host and the firmware derive it as
`CRC32(UID)` ([protocol §11.5](../protocol/README.md)).

### Device-type modules

A device type (e.g. [`../example/thermostat`](../example/thermostat)) is a
dual-target Go module:

- `Config` (a fixed-size struct) is shared by host and firmware.
- `Type() inventory.DeviceType` describes the register table. It is compiled into
  both targets, but the firmware never calls it, so TinyGo's dead-code
  elimination strips it (and the host library it references) from the image.
- The firmware entry point lives behind `//go:build tinygo`.

### Provisioning page

The host and firmware agree on one flash page per device, encoded by the shared
[`config`](config) package (`encoding/binary`, fixed-width, CRC-checked):

```
header  magic | layout | configLen | channel | spreadFactor | address | key
config  the device type's fixed-size Config struct
crc32   CRC-32 (IEEE) over everything before it
```

The firmware reads it once at boot; `config.IsUnprovisioned` distinguishes an
erased page from a corrupt one.

---

## Layout

| Path | Responsibility |
|------|----------------|
| [`cli`](cli) | The `bleriot` command tree (cobra): `cli.Start(Inventory)` plus the `hub`, `provision` and `new` subcommands, and the `Probe` interface (SWD read-UID / write-page) with its `pyocd` implementation. |
| [`inventory`](inventory) | The inventory-as-code model: `Register`/`DeviceType`/`Instance`/`Inventory` and `Validate`. |
| [`config`](config) | The provisioning page codec, shared verbatim with the firmware (host packs it, firmware reads it). |
| [`engine`](engine) | Core protocol logic (§8–§10): XTEA codec per node, `GET`/`SET`/`WATCH`, per-attempt timeout + retransmit, and watch-refresh to keep subscriptions alive within `T_idle`. |
| [`radio`](radio) | Transport-agnostic radio adapter: the `Dongle` interface (a single-channel RF endpoint that can `Send`/`Receive`), plus the hub-side `Radio` (receive loop) and node-side `NodeRadio`. The MCP2210 dongle is one `Dongle`; a future smart dongle would be another. |
| [`radio/mcpdongle`](radio/mcpdongle) | The `Dongle` implementation over an MCP2210 + PAN211x: brings up the radio, runs the per-packet PAN211x register sequence over USB-HID, and drives the status LEDs. |
| [`mcp2210`](mcp2210) | Low-level MCP2210 USB-HID-to-SPI driver (open by `/dev/hidraw*` path or USB serial, chip/GPIO/SPI config, SPI transfers), self-healing against stale/desynced HID responses. |
| [`node`](node) | Host-side node model: a register descriptor (wire ID → name/type/scaling) built from a device type, plus the provisioned identity (address + key). Bridges values to/from the Registry. |
| [`bridge`](bridge) | Connects the engine to the Registry: each register becomes a provider (seeded by `GET`, kept current by `WATCH`), and consumer writes become `SET`. Generic — no per-register knowledge beyond the descriptor. |

---

## Resilience

- The MCP2210 transport **self-heals** from a desynced HID stream: if an aborted
  session left a stale response queued on the device, `command` discards
  mismatched responses until it reads the one matching its request.
- `Watch` subscriptions are persistent intents: they are retained even if the
  initial attempt times out, and re-`WATCH`ed periodically so a node that comes
  up later (or reboots) is resubscribed automatically.

---

## Diagnostics

With `--diagnostics <prefix>` the hub publishes a set of synthetic, **read-only**
Registry registers describing its own RF health, in addition to the device
registers. They are off by default; the prefix namespaces every register (e.g.
`--diagnostics diag` yields `diag.nodes.kitchen.online`). Change requests to these
registers are ignored.

Each per-node and per-dongle traffic counter is exposed twice: a cumulative
`count.*` integer (since hub start) and a `rate.*` float (per second, averaged
over the trailing `--diag-window`, default `30s`). Values are sampled and
republished every 5 s.

**Per node** — `<prefix>.nodes.<node>.<reg>`:

| Register | Type | Meaning |
|----------|------|---------|
| `online` | bool | Node is answering (watch-refresh misses below the liveness threshold). |
| `seen` | int | Unix time (s) of the most recent packet received from the node. |
| `misses` | int | Current consecutive unanswered watch refreshes. |
| `count.rx.all` / `rate.rx.all` | int / float | Packets received from the node. |
| `count.rx.is` / `rate.rx.is` | int / float | `IS` value reports received. |
| `count.rx.acks` / `rate.rx.acks` | int / float | `ACK`s received. |
| `count.rx.corrupt` / `rate.rx.corrupt` | int / float | Packets attributed to the node that failed to decode. |
| `count.tx.all` / `rate.tx.all` | int / float | Packets sent (initial sends and retries). |
| `count.tx.retries` / `rate.tx.retries` | int / float | Retransmissions only. |
| `count.timeouts` / `rate.timeouts` | int / float | Transactions that exhausted all retries with no reply. |

**Per dongle** — `<prefix>.dongle.<channel-name>.<reg>` (labelled by the
channel's `Name`):

| Register | Type | Meaning |
|----------|------|---------|
| `connected` | bool | A physical device is currently open. |
| `reconnects` | int | Times the device has been reopened after the first connect. |
| `since` | int | Unix time (s) of the most recent successful open. |
| `count.tx.all` / `rate.tx.all` | int / float | Transmit attempts. |
| `count.tx.err` / `rate.tx.err` | int / float | Failed transmit attempts (including while offline). |
| `count.rx.all` / `rate.rx.all` | int / float | Packets received. |

