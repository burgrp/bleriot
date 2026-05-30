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
| `nodes` | Directory of per-device node files (see below). |

### Node files

The hub does **not** list nodes in the main config. Instead `nodes` names a
directory, and every `*.json` file in it is one physical node. The file's base
name is the node name. Provisioning a new device is a single file drop — the hub
config is never edited.

```
config.json               # nodes: "nodes"
descriptors/
  thermo.json             # shared, generated per-type descriptor (§11.7)
nodes/
  outdoor.json            # node "outdoor"
  garage.json             # node "garage" — same descriptor, own channel + identity
```

```json
// nodes/outdoor.json
{
  "descriptor": "../descriptors/thermo.json",
  "channel": 37,
  "address": "CCA00002",
  "key": "00112233445566778899AABBCCDDEEFF"
}
```

The `descriptor` path is resolved relative to the node file. See
[protocol §11.9](../protocol/README.md#119-hub-node-files).

---

## Code generation

`bleriot generate` turns a hand-authored JSON spec into the two artifacts
BleRiot needs, from a single run — so firmware and hub can never drift
([protocol §11.7](../protocol/README.md#117-generated-artifacts)):

```sh
./bleriot generate --spec node.json --out out
# out/<node>_gen.go   firmware node code (const wire IDs + RegisterIDs + version)
# out/<node>.json      hub node descriptor (resolved id → name/type/scaling)
```

Flags: `--spec` (input, default `node.json`), `--out` (output dir, default `.`),
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
    "outdoor": "thermometer",
    "main": "switch"
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
