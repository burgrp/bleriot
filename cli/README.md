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

The tool is built with [cobra](https://github.com/spf13/cobra). Today it has one
subcommand, `hub`; `generate` and `provision` are planned.

```
bleriot hub        run the host hub bridge (RF nodes ↔ Registry)
bleriot generate   (planned) generate node code and hub descriptors
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

## Layout

| Path | Responsibility |
|------|----------------|
| [`cmd/`](cmd) | The `bleriot` CLI (cobra): [`main.go`](cmd/main.go) is the root command; [`hub.go`](cmd/hub.go) is the `hub` subcommand — loads config + node files, starts modems, and wires the engine to the bridge. Colored logging via `tint`. |
| [`pkg/engine`](pkg/engine) | Core protocol logic (§8–§10): XTEA codec per node, `GET`/`SET`/`WATCH`, per-attempt timeout + retransmit, and watch-refresh to keep subscriptions alive within `T_idle`. |
| [`pkg/modem`](pkg/modem) | Host-side client for a single modem over one serial port: wraps the [link protocol](../hub/link/README.md) and exposes configure/send/receive. `Port` is a self-healing variant that survives transport loss and reconnects automatically. |
| [`pkg/node`](pkg/node) | Host-side node model: the generated descriptor (wire ID → name/type/scaling) plus the separately provisioned identity (address + key). Bridges values to/from the Registry. |
| [`pkg/bridge`](pkg/bridge) | Connects the engine to the Registry: each register becomes a provider (seeded by `GET`, kept current by `WATCH`), and consumer writes become `SET`. Generic — no per-register knowledge beyond the descriptor. |

---

## Resilience

- The modem `Port` starts even when the radio is **disconnected** and reconnects
  with backoff; `Send` returns `ErrDisconnected` until a transport is available.
- `Watch` subscriptions are persistent intents: they are retained even if the
  initial attempt times out, and re-`WATCH`ed periodically so a node that comes
  up later (or reboots) is resubscribed automatically.
