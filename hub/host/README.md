# hub/host — BleRiot host bridge

The **host** is the Linux-SBC half of the BleRiot hub. It owns all protocol
intelligence — per-node XTEA keys, node descriptors, retries/timeouts, push
subscription bookkeeping, and the Registry client — and drives one or more
"dumb radio modems" ([`hub/fw`](../fw/README.md)) over serial.

For every BleRiot register it acts as a [Registry](https://github.com/burgrp/reg)
provider: it publishes the register's value and turns consumer change requests
into BleRiot `SET` operations.

> Module path: `hub/host`. See the [protocol spec](../../protocol/README.md) for
> the wire format and transaction semantics this implements.

---

## Build & run

```sh
make build              # → ./hub
make run                # go run ./cmd/hub
make test               # go test ./...
make vet
./hub -config hub.json
```

If `registry` is empty in the config, the `REGISTRY` environment variable is used.

---

## Configuration

The hub is configured with a single JSON file (`-config`). Paths in it are
resolved relative to the config file's directory.

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
    { "device": "/dev/ttyUSB0", "channel": 37 }
  ],
  "nodesDir": "nodes"
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
| `nodesDir` | Directory of per-device node files (see below). |

### Node files

The hub does **not** list nodes in the main config. Instead `nodesDir` names a
directory, and every `*.json` file in it is one physical node. The file's base
name is the node name. Provisioning a new device is a single file drop — the hub
config is never edited.

```
hub.json                  # nodesDir: "nodes"
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
[protocol §11.9](../../protocol/README.md#119-hub-node-files).

---

## Packages

| Package | Responsibility |
|---------|----------------|
| [`cmd/hub`](cmd/hub) | Entry point: loads config + node files, starts modems, wires the engine to the bridge. Colored logging via `tint`. |
| [`engine`](engine) | Core protocol logic (§8–§10): XTEA codec per node, `GET`/`SET`/`WATCH`, per-attempt timeout + retransmit, and watch-refresh to keep subscriptions alive within `T_idle`. |
| [`modem`](modem) | Host-side client for a single modem over one serial port: wraps the [link protocol](../link/README.md) and exposes configure/send/receive. `Port` is a self-healing variant that survives transport loss and reconnects automatically. |
| [`node`](node) | Host-side node model: the generated descriptor (wire ID → name/type/scaling) plus the separately provisioned identity (address + key). Bridges values to/from the Registry. |
| [`bridge`](bridge) | Connects the engine to the Registry: each register becomes a provider (seeded by `GET`, kept current by `WATCH`), and consumer writes become `SET`. Generic — no per-register knowledge beyond the descriptor. |

---

## Resilience

- The modem `Port` starts even when the radio is **disconnected** and reconnects
  with backoff; `Send` returns `ErrDisconnected` until a transport is available.
- `Watch` subscriptions are persistent intents: they are retained even if the
  initial attempt times out, and re-`WATCH`ed periodically so a node that comes
  up later (or reboots) is resubscribed automatically.
