# site — BleRiot host library

`lib/site` is the host (Linux-SBC) half of the BleRiot hub, packaged as a
**library** that a site repository drives with **inventory-as-code**. It owns all
protocol intelligence — per-node XTEA keys, register tables, retries/timeouts,
push subscription bookkeeping, and the Registry client — and drives the radio
directly over one or more USB dongles (an [MCP2210](../../usb) USB-to-SPI bridge
driving a PAN211x; the dongle has no microcontroller and no firmware).

For every BleRiot register the hub acts as a
[Registry](https://github.com/burgrp/reg) provider: it publishes the register's
value and turns consumer change requests into BleRiot `SET` operations.

> Import path: `github.com/burgrp/bleriot/lib/site` (part of the single `lib`
> module). See the [protocol spec](../README.md) for the wire format and
> transaction semantics this implements.

There is **no JSON configuration and no code generation**. A deployment is a Go
program: it declares an [`inventory.Inventory`](../shared/inventory) and hands it
to [`cli.Start`](cli), which builds the `bleriot` command tree (cobra) around
it.

```go
package main

import (
	"github.com/burgrp/bleriot/lib/site/cli"
	"github.com/burgrp/bleriot/lib/shared/inventory"

	bob "github.com/burgrp/bleriot/example/bob/spec"
)

func main() {
	cli.Start(inventory.Inventory{
		{
			Name:    "bob",
			UID:     [12]byte{ /* MCU unique ID */ },
			Key:     [16]byte{ /* XTEA key */ },
			Channel: inventory.Channel{Name: "far", Number: 37},
			Type:    bob.Type(),
			Config:  bob.Config{DefaultRedPeriod: 500, DefaultGreenPeriod: 100},
		},
	})
}
```

A complete, runnable site binary lives in [`../../example/bob`](../../example/bob),
behind `//go:build !tinygo` in the same flat `package main` as the node firmware.

---

## Commands

`cli.Start` provides three subcommands:

```
hub  bridge the inventory's RF nodes to the Registry
gen  emit a device's baked-in identity + config as firmware source
new  read an attached device's UID and print an Instance stub
```

```sh
cd ../../example/bob
go run . hub --registry http://localhost:8080
go run . --debug hub                           # verbose: shows radio traffic
go run . gen                                  # generate the firmware provisioning source
go run . new                                  # onboard a brand-new device
```

The hub discovers the connected USB radio dongles automatically and assigns them
to the channels the inventory uses, so there is no dongle flag. It always starts,
even with no dongle connected: each RF channel stays offline until a dongle
becomes available. Dongles are assigned dynamically — plugging one in brings up
an orphan channel, and unplugging it frees the device to be reassigned, possibly
to a different channel, when it returns. A discovered dongle is pinned by its USB
serial where it has one, so a self-healing radio re-finds the same device after a
replug.

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
| `--diagnostics` | — (off) | Publish hub-synthesised diagnostic registers under this Registry namespace prefix (e.g. `diag`). Empty disables them. See [Diagnostics](#diagnostics). |
| `--diag-window` | `30s` | Averaging window for the diagnostic `rate.*` registers. |

### `new` flags

`new` talks to the attached device over SWD via `pyocd` to read its UID. The
chip-specific details (pyocd target, UID memory address) are a property of the
device's MCU and are declared once on the device type's `Chip` field, so the
command takes a single flag:

| Flag | Default | Meaning |
|------|---------|---------|
| `--chip` | — | Chip to read over SWD. Required only when the inventory declares more than one chip; otherwise the sole chip is selected automatically. |

A `Chip` bundles the build and flash targets — `TinygoTarget` (tinygo
`--target`), `PyocdTarget` (pyocd target name), `CmsisPack` (pyocd CMSIS pack) —
and `UIDAddr` (memory address of the 12-byte MCU unique ID). `inventory.PY32F030x8`
(`py32f030_64k_8k`/`py32f030x8`, UID `0x1FFF0E00`) is built in; declare a
`Chip{...}` on a device type to support other MCUs. `--chip` accepts a built-in
chip name even on an empty inventory, so the very first device can be onboarded
with `new`.

`new` reads the UID of a device not yet in the inventory and prints a paste-ready
`inventory.Instance{}` stub to add to the source. `gen` bakes an inventory
instance's identity (the RF address derived as `CRC32(UID)`, key, channel, spread
factor) and `Config` into a generated Go file the firmware build compiles in; it
touches no hardware and emits to stdout, so it is mainly for inspection —
`bleriot make` (below) writes the same file as part of building.

`make [name] [make-args...]` builds or flashes a device straight from the hub: it
locates the device type's firmware source (the nearest Makefile above its `Config`
package, or `--root`), writes the baked-in identity + config into it as
`main_gen.go`, then runs GNU make there with the chip's
`TinygoTarget`/`PyocdTarget`/`CmsisPack` and a `BLERIOT_MAKE` sentinel injected as
make variables. Name the instance, or omit it to use the sole one. So
`bleriot make bob flash` bakes bob's hub-owned identity and flashes it, without
the firmware directory needing its own inventory. The example Makefile also has a
guard: running `make flash` directly (without the sentinel) bounces the goal back
through `go run . make`, so a single-device dir builds correctly either way. It
needs the firmware source and toolchain (make, tinygo, pyocd) present, so run it
from a hub checkout.

### USB access

The MCP2210 dongle appears as a `/dev/hidraw*` node that is root-only by
default, so the `hub` would otherwise need `sudo`. Install the udev rule shipped
with the dongle hardware design — [`../../usb/99-bleriot-mcp2210.rules`](../../usb/99-bleriot-mcp2210.rules) —
to grant access to the local desktop user and the `plugdev` group instead:

```sh
sudo cp ../../usb/99-bleriot-mcp2210.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
sudo udevadm trigger --subsystem-match=hidraw --action=change
```

Re-plug the dongle if it was already connected, and make sure your user is in
the `plugdev` group (`id` should list it; otherwise `sudo usermod -aG plugdev
$USER` and re-login). The rule matches the MCP2210 by its USB ID `04d8:00de`.

> Reading the UID over SWD (`new`) drives the SWD probe through
> `pyocd`, which has its own access requirements independent of this rule.

---

## Inventory model

The [`inventory`](../shared/inventory) package is the type-safe model of a
deployment. It is part of [`lib/shared`](../shared) — shared with the firmware so
both sides agree on register tags and the identity format — not host-only
code.

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
`CRC32(UID)` ([protocol §11.5](../README.md)).

### Device-type modules

A device type (e.g. [`../../example/bob`](../../example/bob)) is its own,
dual-target Go module:

- `Config` is shared by host and firmware.
- `Type() inventory.DeviceType` describes the register table. It is compiled into
  both targets, but the firmware never calls it, so TinyGo's dead-code
  elimination strips it (and the `inventory` package it references) from the
  image.
- One flat `package main` holds both entry points, selected by build tag: the
  firmware (`//go:build tinygo`) drives the shared [`lib/node`](../node) runtime
  over the radio, and the example host hub (`//go:build !tinygo`) declares the
  inventory and hands it to [`lib/site/cli`](cli). So a single module is both the
  node firmware and its example hub.

### Generated provisioning file

There is no provisioning page in flash. Instead, a small generated Go file
(`//go:build tinygo`, gitignored) bakes one inventory instance's identity and
config into the firmware image. `bleriot make` writes it as part of building, and
`bleriot gen` emits the same source to stdout for inspection:

```go
//go:build tinygo

// Code generated by "bleriot gen"; DO NOT EDIT.
package main

func main() {
	bleriotMain(node.Provisioning{
		Address:      [4]byte{ /* CRC32(UID) */ },
		Key:          [16]byte{ /* XTEA key */ },
		Channel:      37,
		SpreadFactor: 0,
	}, spec.Config{ /* device config */ })
}
```

`node.Provisioning` (address, key, channel, spread factor) lives in the shared
[`lib/node`](../node) package; the `Config` literal is rendered from the
inventory value with `%#v`. The firmware's hand-written `bleriotMain` receives
both. At boot the firmware self-checks that its MCU UID hashes to the baked-in
address, so an image flashed to the wrong board refuses to join the network.

---

## Layout

| Path | Responsibility |
|------|----------------|
| [`cli`](cli) | The `bleriot` command tree (cobra): `cli.Start(Inventory)` plus the `hub`, `gen`, `make` and `new` subcommands, and the `Probe` interface (SWD read-UID) with its `pyocd` implementation. |
| [`../shared/inventory`](../shared/inventory) | The inventory-as-code model: `Register`/`DeviceType`/`Instance`/`Inventory` and `Validate`. Shared with the firmware. |
| [`../shared/config`](../shared/config) | Identity primitives and constants (address/key/UID lengths, spread factor), shared verbatim with the firmware. |
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
`--diagnostics diag` yields `diag.node.kitchen.online`). Change requests to these
registers are ignored.

Each per-node and per-dongle traffic counter is exposed twice: a cumulative
`count.*` integer (since hub start) and a `rate.*` float (per second, averaged
over the trailing `--diag-window`, default `30s`). Values are sampled and
republished every 5 s.

**Per node** — `<prefix>.node.<node>.<reg>`:

The `<node>` segment is always a single path component: any `.` in the instance
name is replaced by `_` (e.g. instance `basement.fan` →
`<prefix>.node.basement_fan.rate.rx.all`, never `…node.basement.fan.rate.rx.all`),
so the node name stays at a fixed position for selectors like Grafana.

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
| `up` | int | Unix time (s) of the most recent successful open (start of the current uptime). |
| `down` | int | Unix time (s) of the most recent disconnect (start of the current outage). |
| `count.tx.all` / `rate.tx.all` | int / float | Transmit attempts. |
| `count.tx.err` / `rate.tx.err` | int / float | Failed transmit attempts (including while offline). |
| `count.rx.all` / `rate.rx.all` | int / float | Packets received. |

