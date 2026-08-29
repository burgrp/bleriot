# site — BleRiot host library

`lib/site` is the host (Linux-SBC) half of the BleRiot hub, packaged as a
**library** that a site repository drives with **inventory-as-code**. It owns all
protocol intelligence — per-node XTEA keys, register tables, retries/timeouts,
channel sweep scheduling, and the Registry client — and drives the radio
directly over one or more USB dongles (an [MCP2210](../../dongle/mcp2210) USB-to-SPI bridge
driving a PAN211x; the dongle has no microcontroller and no firmware).

For every BleRiot register the hub acts as a
[Registry](https://github.com/burgrp/reg) provider: it publishes the register's
value and turns consumer change requests into BleRiot `SET` operations.

> Import path: `github.com/burgrp/bleriot/lib/site` (part of the single `lib`
> module). See the [protocol spec](../README.md) for the wire format and
> transaction semantics this implements.

There is **no JSON configuration or generated register descriptor**. A
deployment is a Go program: it declares an
[`inventory.Inventory`](../shared/inventory) and hands it to
[`cli.Start`](cli), which builds the `bleriot` command tree (cobra) around it.
Only the TinyGo firmware entry point is generated, to bake one instance's
identity and config into its image.

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
      Address: [4]byte{ /* random RF address */ },
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

`cli.Start` provides four subcommands:

```
hub  bridge the inventory's RF nodes to the Registry
gen  emit a device's baked-in identity + config as firmware source
make build/flash firmware with a device's identity + config baked in
new  generate a random identity and print an Instance stub
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
| `--sweep-interval` | `1s` | Target period for a complete polling round on each RF channel. Slow rounds slip and never overlap. |
| `--ttl` | `30s` | Registry provider TTL. |
| `--diagnostics` | — (off) | Publish hub-synthesised diagnostic registers under this Registry namespace prefix (e.g. `diag`). Empty disables them. See [Diagnostics](#diagnostics). |
| `--diag-interval` | `1s` | How often changed diagnostics are coalesced into one Registry batch. |

A `Chip` bundles the build and flash targets — `TinygoTarget` (tinygo
`--target`), `PyocdTarget` (pyocd target name), and `CmsisPack` (pyocd CMSIS
pack). The `puya` package
provides built-in profiles for PY32F002A/B, every PY32F003 density, and every
PY32F030 density; declare a `Chip{...}` on a device type to support other MCUs.

`new` generates a random nonzero RF address and XTEA key and prints a paste-ready
`inventory.Instance{}` stub. It is entirely offline and requires no device or
probe. `gen` bakes an inventory instance's stored address, key, channel, spread
factor and `Config` into a generated Go file the firmware build compiles in; it
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
with the dongle hardware design — [`../../dongle/mcp2210/99-bleriot-mcp2210.rules`](../../dongle/mcp2210/99-bleriot-mcp2210.rules) —
to grant access to the local desktop user and the `plugdev` group instead:

```sh
sudo cp ../../dongle/mcp2210/99-bleriot-mcp2210.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
sudo udevadm trigger --subsystem-match=hidraw --action=change
```

Re-plug the dongle if it was already connected, and make sure your user is in
the `plugdev` group (`id` should list it; otherwise `sudo usermod -aG plugdev
$USER` and re-login). The rule matches the MCP2210 by its USB ID `04d8:00de`.

---

## Inventory model

The [`inventory`](../shared/inventory) package is the type-safe model of a
deployment. It is part of [`lib/shared`](../shared) — shared with the firmware so
both sides agree on register tags and the identity format — not host-only
code.

- **`Register`** — one register of a device type. Its `Tag` (a `uint16`, like a
  protobuf field number) is its permanent wire identity: unique and non-zero
  within the device type, never reused once retired. Slice order is irrelevant.
  `Type` selects the default Registry value conversion; an optional
  `Conversion` supplies fallible `Decode` and `Encode` functions. `ReadOnly`
  registers may define `Decode` alone, and the bridge ignores all consumer
  writes to them.
- **`DeviceType`** — a shared, per-type register table (`Name` + `Registers`),
  authored once in the device-type module and returned by its `Type()` function.
- **`Instance`** — one physical device: its `Name`, random RF `Address`, XTEA `Key`, RF
  `Channel`, device `Type`, and device-specific `Config`.
- **`Channel`** — an RF channel `Number` together with the `SpreadFactor` every
  node on it uses. Spreading factor is a property of the channel (a dongle
  transmits one factor at a time), so binding the two makes it impossible to give
  two nodes on one channel different factors. Declare each channel once and share
  that value across instances; the zero `SpreadFactor` is the highest-range S8.
  Each channel also has a required, unique `Name` (e.g. `"far"`) that scopes its
  per-dongle [diagnostic registers](#diagnostics).
- **`Inventory`** — the full set of devices. `Validate()` enforces unique
  non-zero tags per device type, unique instance names and nonzero addresses, one spreading factor per
  channel, and a non-empty, one-to-one mapping between channel numbers and names.

The RF address and key are generated by `new`, stored in inventory, and baked
into firmware by `make` ([protocol §11.5](../README.md)).

### Device-type modules

A device type (e.g. [`../../example/bob`](../../example/bob)) is its own,
dual-target Go module:

- `Config` is shared by host and firmware.
- `Type() inventory.DeviceType` describes the register table. It is compiled into
  both targets, but the firmware never calls it, so TinyGo's dead-code
  elimination strips it (and the `inventory` package it references) from the
  image. Register conversion functions therefore execute on the hub only, even
  when they use floating-point sensor mathematics such as an NTC beta equation.
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
    Address:      [4]byte{ /* random RF address */ },
		Key:          [16]byte{ /* XTEA key */ },
		Channel:      37,
		SpreadFactor: 0,
	}, spec.Config{ /* device config */ })
}
```

`node.Provisioning` (address, key, channel, spread factor) lives in the shared
[`lib/node`](../node) package; the `Config` literal is rendered from the
inventory value with `%#v`. The firmware's hand-written `bleriotMain` receives
both.

---

## Layout

| Path | Responsibility |
|------|----------------|
| [`cli`](cli) | The `bleriot` command tree (cobra): `cli.Start(Inventory)` plus the `hub`, `gen`, `make` and offline `new` subcommands. |
| [`../shared/inventory`](../shared/inventory) | The inventory-as-code model: `Register`/`DeviceType`/`Instance`/`Inventory` and `Validate`. Shared with the firmware. |
| [`../shared/conversion`](../shared/conversion) | Hub-side `Scale` and `Linear` factories for writable register conversions. The code is referenced by shared device specs but stripped from firmware as unreachable. |
| [`../shared/conversion/ntc`](../shared/conversion/ntc) | Read-only raw-ADC to Celsius conversion using an NTC thermistor's beta model. |
| [`../shared/puya`](../shared/puya) | Puya PY32 chip profiles and per-family memory-map constants. Shared with the firmware. |
| [`../shared/config`](../shared/config) | Identity primitives and constants (address/key lengths, spread factor), shared verbatim with the firmware. |
| [`engine`](engine) | Core protocol logic: XTEA codec per node, serialized `GET`/`VALUE` and `SET`/`ACK` transactions per channel, and per-attempt timeout + retransmit. |
| [`radio`](radio) | Transport-agnostic radio adapter: the `Dongle` interface (a single-channel RF endpoint that can `Send`/`Receive`), plus the hub-side `Radio` (receive loop) and node-side `NodeRadio`. The MCP2210 dongle is one `Dongle`; a future smart dongle would be another. |
| [`radio/mcpdongle`](radio/mcpdongle) | The `Dongle` implementation over an MCP2210 + PAN211x: brings up the radio, runs the per-packet PAN211x register sequence over USB-HID, and drives the status LEDs. |
| [`mcp2210`](mcp2210) | Low-level MCP2210 USB-HID-to-SPI driver (open by `/dev/hidraw*` path or USB serial, chip/GPIO/SPI config, SPI transfers), self-healing against stale/desynced HID responses. |
| [`node`](node) | Host-side node model: a register descriptor (wire ID → name/type/access/conversion) built from a device type, plus the provisioned identity (address + key). |
| [`bridge`](bridge) | Connects the engine to the Registry: each register becomes a provider updated by complete per-channel polling sweeps, and writable consumer requests become `SET`. Decode failures publish `nil`; read-only writes are ignored. Generic — no per-register knowledge beyond the descriptor. |

---

## Runtime behavior

Each inventory RF channel is an independent half-duplex transaction group. The
engine keeps one persistent lane per channel and permits one in-flight `GET` or
`SET`, including retries and its reply wait. A physical radio can disconnect and
be replaced without creating a second lane. Different channels run
concurrently.

The host accepts a response only while it owns that channel and only when the
source, register, and response type match the active transaction. No transaction
token is carried on the wire. The default 50 ms per-attempt timeout and three
retransmissions allow up to four sends. A first-attempt success releases the
channel immediately. After a retry succeeds, after a final timeout, or after
caller cancellation following a successful send, the engine keeps the channel
for a bounded response drain and consumes late packets before admitting the next
transaction. Caller cancellation cannot abort that drain. Send errors are not
retried.

The radio reports the turnaround guard encoded on every request. A node waits
that guard before every response. Channel setup fails if the guard plus 10 ms of
reply headroom exceeds `--timeout`. The response drain lasts the larger of 10 ms
and that guard-plus-headroom window.

### Polling scheduler and fairness

The bridge starts one scheduler worker per channel. A round visits every node
on that channel and polls all of each node's registers in descriptor order. The
`--sweep-interval` is the target period for the complete round: a faster round
waits out the remainder, while an overloaded round makes the next start slip
and never overlaps itself.

The first node rotates each round. Every snapshotted node completes before any
is repeated, preventing the same node from always leading an overloaded
channel. Registry-originated `SET` transactions share the channel lane with
polls.

### Registry publication and liveness

Every provider starts at `nil`. A successful `GET` publishes the converted
value; protocol `NULL` publishes `nil` without conversion. Conversion failure
logs the raw value and also publishes `nil`, but still counts as successful
node contact. An isolated failed GET leaves that register's last publication
unchanged.

Liveness is per node. Three consecutive sweeps with no successful transaction
mark the node offline and publish all of its registers as `nil` once. Any
successful GET or received SET acknowledgment resets the failure count and clears
the offline state; current values return through successful GETs.

Writable Registry requests are converted and sent as absolute `SET`
assignments. A Registry `nil` request becomes a `NULL` assignment. Requests for
read-only registers, including `nil`, are ignored. An `ACK` never updates the
provider optimistically: it confirms only that the node received the request.
Normal polling rounds observe and publish the current value, which may remain
old or intermediate while an asynchronous action settles. Provider TTL defaults
to 30 seconds, and pending publications are coalesced so a slow Registry consumer
receives the latest value.

---

## Resilience

- The MCP2210 transport **self-heals** from a desynced HID stream: if an aborted
  session left a stale response queued on the device, `command` discards
  mismatched responses until it reads the one matching its request.
- MCP2210 HID commands and SPI progress are bounded. A silent USB device or a
  bridge that remains in SPI-in-progress state is failed instead of blocking the
  receive loop; receive-side transport failures close and reopen the dongle just
  like send failures.
- Polling rounds continue after timeouts, so a node that comes up later or
  reboots is discovered again and resumes publication.

---

## Diagnostics

With `--diagnostics <prefix>` the hub publishes a set of synthetic, **read-only**
Registry registers describing its own RF health, in addition to the device
registers. They are off by default. Schema 8 publishes process-lifetime hub,
per-node `transaction.get.*`/`transaction.set.*`, packet, and per-channel
connection metrics. Rates and windowed increases are derived by consumers.

Hot paths update atomics without Registry I/O. Every `--diag-interval` (default
1 s), one publisher snapshots the complete catalog and sends at most one batch
containing changed values plus a distributed TTL-refresh cohort. A failed batch
is retried on the next interval.

See [BleRiot Diagnostic Metrics](DIAGNOSTICS.md) for the complete schema 8 path
catalog, accounting identities and examples, latency formulas, escaping,
cardinality, Prometheus queries, and restart semantics.

