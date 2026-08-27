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
| `--refresh` | `15s` | How often active `WATCH` subscriptions are refreshed (§10). |
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
| [`engine`](engine) | Core protocol logic (§8–§10): XTEA codec per node, `GET`/`SET`/`WATCH`, per-attempt timeout + retransmit, and watch-refresh to keep subscriptions alive within `T_idle`. |
| [`radio`](radio) | Transport-agnostic radio adapter: the `Dongle` interface (a single-channel RF endpoint that can `Send`/`Receive`), plus the hub-side `Radio` (receive loop) and node-side `NodeRadio`. The MCP2210 dongle is one `Dongle`; a future smart dongle would be another. |
| [`radio/mcpdongle`](radio/mcpdongle) | The `Dongle` implementation over an MCP2210 + PAN211x: brings up the radio, runs the per-packet PAN211x register sequence over USB-HID, and drives the status LEDs. |
| [`mcp2210`](mcp2210) | Low-level MCP2210 USB-HID-to-SPI driver (open by `/dev/hidraw*` path or USB serial, chip/GPIO/SPI config, SPI transfers), self-healing against stale/desynced HID responses. |
| [`node`](node) | Host-side node model: a register descriptor (wire ID → name/type/access/conversion) built from a device type, plus the provisioned identity (address + key). |
| [`bridge`](bridge) | Connects the engine to the Registry: each register becomes a provider (seeded by `GET`, kept current by `WATCH`), and writable consumer requests become `SET`. Decode failures publish `nil`; read-only writes are ignored. Generic — no per-register knowledge beyond the descriptor. |

---

## Resilience

- The MCP2210 transport **self-heals** from a desynced HID stream: if an aborted
  session left a stale response queued on the device, `command` discards
  mismatched responses until it reads the one matching its request.
- MCP2210 HID commands and SPI progress are bounded. A silent USB device or a
  bridge that remains in SPI-in-progress state is failed instead of blocking the
  receive loop; receive-side transport failures close and reopen the dongle just
  like send failures.
- `Watch` subscriptions are persistent intents: they are retained even if the
  initial attempt times out, and re-`WATCH`ed periodically so a node that comes
  up later (or reboots) is resubscribed automatically.

---

## Diagnostics

With `--diagnostics <prefix>` the hub publishes a set of synthetic, **read-only**
Registry registers describing its own RF health, in addition to the device
registers. They are off by default. Schema version 3 exposes cumulative counters
only; Prometheus/Grafana derives rates and increases over any requested time
range instead of consuming fixed-window gauges. The engine keeps detailed
per-operation accounting internally, while the Registry exports a compact
29-register summary per node suitable for larger fleets.

Hot paths update atomics without Registry I/O. Every `--diag-interval` (default
1 s), one publisher snapshots the complete catalog and sends at most one batch
containing changed values plus a distributed TTL-refresh cohort. A failed batch
remains unsent and is retried on the next interval. This bounds publication
latency without issuing one request per counter; every unchanged value is
refreshed within half of its TTL.

**Hub publisher** — `<prefix>.hub.main.<reg>`:

| Register | Meaning |
|----------|---------|
| `schema.version` | Diagnostics schema version (`3`). |
| `process.started` / `process.heartbeat` | Hub start and latest snapshot Unix times. |
| `publisher.batch.success` / `publisher.batch.error` | Successful and failed Registry batches. |
| `publisher.values.sent` / `publisher.values.coalesced` | Values sent and unchanged values omitted. |
| `publisher.last.success` / `publisher.last.error` | Last successful and failed publish Unix times. |
| `latency.success.count` / `.microseconds` | Fleet-wide successful transaction count and summed latency. |
| `latency.success.bucket.le_<bound>` | Fleet-wide cumulative latency histogram (`0.025`, `0.05`, `0.1`, `0.2`, `0.5`, `1`, `2`, `+Inf` seconds). |

**Per node** — `<prefix>.node.<node>.<reg>`:

The `<node>` segment is always one path component: `_` is escaped as `__`, then
`.` is replaced by `_`, so the node name stays at a fixed selector position
without collisions between names such as `a.b` and `a_b`.

Liveness state codes are `0` unknown, `1` online, `2` suspect, and `3` offline.

| Register | Type | Meaning |
|----------|------|---------|
| `liveness.state` / `liveness.since` | int | Current state code and transition Unix time. |
| `liveness.probe.misses` | int | Current consecutive unanswered refresh probes. |
| `liveness.transition.offline` | int | Cumulative transitions into offline. |
| `packet.last.valid` | int | Latest authenticated, semantically valid packet Unix time. |
| `packet.rx.total` / `packet.rx.valid` | int | Raw packets attributed by cleartext source and packets passing validation. |
| `packet.rx.push` / `packet.rx.orphan` | int | Valid spontaneous pushes and unmatched `IS`/`ACK` packets. |
| `packet.rx.invalid` | int | Decode, source, or type validation failures. |
| `packet.rx.unknown_register` | int | Valid packets carrying a register absent from the provisioned descriptor. |
| `packet.tx.success` / `packet.tx.error` | int | Actual radio send outcomes. |
| `packet.push_ack.failure` | int | Push acknowledgements that failed to send or had no radio. |
| `latency.success.count` / `.microseconds` | int | Successful transaction count and summed latency for per-node mean latency. |

Every known-node invocation increments exactly one aggregate
`transaction.all.outcome.<outcome>` counter: `success_first`, `success_retry`,
`timeout`, `send_error`, `canceled`, `busy`, or `no_radio`. These provide exact
reliability denominators. `transaction.<operation>.invocation.total` preserves
the traffic mix for `get`, `set`, `watch`, `unwatch`, and `refresh`, while
`transaction.all.attempt.retry` counts retransmissions. Detailed combinations
of operation and outcome remain available inside the process but are not
published for every node.

**Per channel** — `<prefix>.channel.<channel-name>.<reg>`:

Channel state codes are `0` offline, `1` connected, and `2` closed.

| Register | Type | Meaning |
|----------|------|---------|
| `state` / `state.since` | int | Current channel state and transition Unix time. |
| `connection.open.attempt` / `.success` / `.error` | int | Physical-device open outcomes. |
| `connection.open.last_attempt` / `.last_error` | int | Latest open attempt and failure Unix times. |
| `connection.connected_at` / `.disconnected_at` | int | Latest connection transition Unix times. |
| `connection.disconnect.total` | int | Disconnects after an established connection. |
| `connection.disconnect.send_error` / `.receive_error` | int | Disconnect cause counters. |
| `packet.tx.attempt` / `.success` / `.offline` / `.error` | int | Channel transmit outcomes, separating no device from connected-device failure. |
| `packet.rx.success` / `.error` | int | Received packets and receive transport failures. |

