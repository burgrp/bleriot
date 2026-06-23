# BleRiot Protocol Specification

Hardware-independent specification for the BleRiot IoT register protocol.

> This is also the README of the [`lib`](.) module
> (`github.com/burgrp/bleriot/lib`), the single library that implements this
> specification. The module is split into:
>
> - [`shared`](shared) — neutral, dependency-free, build-tag-free packages used by
>   both firmware and host: `protocol` (packet codec + XTEA, §4–§8), `config`
>   (the provisioning page, §11.5) and `inventory` (the inventory-as-code model,
>   §11).
> - [`node`](node) — the firmware-side runtime (receive/dispatch loop, §7–§8).
> - [`site`](site) — the host hub library; see [site/README.md](site/README.md).

---

## 1. Overview

BleRiot is a simple request/response protocol for reading and writing named integer registers on IoT nodes. A single hub polls one or more nodes. Nodes send unsolicited IS packets only for registers the hub has subscribed to via WATCH.

- **Topology:** star — one hub, many nodes
- **Initiator:** hub only (except subscribed push packets)
- **Register type:** `int32` (signed 32-bit integer)

---

## 2. RF Physical Layer

These parameters are mandatory for all BleRiot-compatible radio implementations:

| Parameter        | Value                                                                   |
|------------------|-------------------------------------------------------------------------|
| Channel          | Per-node, provisioned via §11 (e.g. channel 10 = 2440 MHz)             |
| Sync word        | Destination device address (4 bytes, little-endian) — see §3           |
| Data rate        | 250 kbps                                                                |
| Modulation       | GFSK                                                                    |
| Packet format    | BLE-compatible (preamble, sync word, PDU, 3-byte CRC, whitening)       |
| PDU size         | 13 bytes (fixed)                                                        |

Each node operates on a single channel defined in its provisioning page (§11.5). The hub may have multiple radio interfaces, each assigned to a different channel, allowing nodes to be grouped by channel for spectrum spread or logical partitioning.

The RF sync word for each transmission is set to the 4-byte destination device address. Radio hardware that supports address/pipe filtering must configure its receive address to the device's own address, so only packets destined for that device are passed to the protocol layer. The 32-bit address space makes accidental collision with foreign RF traffic negligible.

---

## 3. Device Addressing

Each device has a **4-byte address** derived from a hardware unique identifier (e.g. CRC32 of the MCU UID, or CRC32 of a host MAC address). Addresses are treated as opaque 32-bit values.

Reserved address: `0x00000000` — must not be assigned to any device.

---

## 4. Packet Format

All packets share the same fixed 13-byte structure:

```
Offset  Size  Field
──────  ────  ─────────────────────────────────────────────────
0       4     SRC   — source device address (little-endian, plaintext)
4       1     VER   — packet format version (plaintext)
5       8     BLOCK — XTEA encrypted block (see §5):
                        TYPE  (1 byte)  — packet type (see §6)
                        FLAGS (1 byte)  — options (see §7)
                        REG   (2 bytes) — register address (uint16, little-endian)
                        VALUE (4 bytes) — int32, little-endian (zero in GET/WATCH)
──────  ────
Total: 13 bytes
```

The destination is not carried in the payload. It is encoded as the RF sync word (§2), which the receiver's hardware uses for filtering. A received packet is therefore always addressed to the receiving device.

SRC and VER are plaintext so the receiver can look up the sender's shared key and validate the packet format before decrypting BLOCK.

All multi-byte fields inside BLOCK are **little-endian**.

---

## 5. Security

All packets are encrypted with **XTEA** using the node's shared key (provisioned via §10). The 8-byte BLOCK field contains the payload: TYPE, FLAGS, REG, and VALUE. There is no per-packet nonce in this format.

The version byte is used for wire-format compatibility and does not add confidentiality or replay protection.

The hub decrypts each received packet using the shared key associated with the SRC address. Packets from unknown addresses are silently discarded.

---

## 6. FLAGS Byte

The FLAGS byte is direction-dependent. Bits 7–1 are GUARD on hub → node
requests; on node → hub replies that range is unused except for bit 1, which
carries PUSH.

```
hub → node:
  Bit 7–1  — GUARD: reply turnaround guard, 0–127 ms
  Bit 0    — NULL: VALUE field is absent; register has no value

node → hub:
  Bit 1    — PUSH: this IS is an unsolicited push and must be ACKed (§8.3)
  Bit 0    — NULL: VALUE field is absent; register has no value
```

When NULL=1 the VALUE field is undefined and must be ignored by the receiver.

PUSH=1 marks an IS that a node sent on its own initiative (a WATCH change
notification, §8.3) rather than as the reply to a request. Because nothing on the
hub is waiting for it, a lost push would otherwise go unnoticed; PUSH tells the
hub to acknowledge it so the node can retransmit until it lands (§9). PUSH is
meaningful only on node → hub IS packets and is clear on every solicited reply.

GUARD is the number of milliseconds a node waits, after receiving a request, before it transmits its reply (§9). A half-duplex hub radio needs time to switch from transmit back to receive after sending a request; a fast node that replied immediately would answer into a window when the hub is not yet listening, and the reply would be lost. The hub sets GUARD on every request from the turnaround time of the radio that carries it (a slower radio asks for a larger guard), and a node honours it before any IS or ACK reply. Replies (node → hub) carry GUARD = 0, and a GUARD of 0 means reply immediately. GUARD must be smaller than the hub's response timeout `T_timeout` (§9).

---

## 7. TYPE Byte

| Value | Name  | Sender | Description                                        |
|-------|-------|--------|----------------------------------------------------|
| 0x00  | GET   | hub    | Read current register value (one-shot)             |
| 0x01  | SET   | hub    | Write register value                               |
| 0x02  | IS    | node   | Current register value (reply to GET/WATCH, or push)|
| 0x03  | WATCH | hub    | Subscribe (VALUE=1) or unsubscribe (VALUE=0)       |
| 0x04  | ACK   | both   | Acknowledges a SET (node) or a push (hub); no value |

Receivers must ignore packets with unknown TYPE values.

A node sends IS with FLAGS.NULL=1 when a register has no value (e.g. sensor not yet ready, hardware fault, or explicitly unset).

ACK is used in two directions and always carries no value (VALUE=0):

- **node → hub** confirms a SET was received. A write may be applied
  asynchronously, so the node does not report a result inline; the hub observes
  the resulting value through a WATCH subscription or a subsequent GET.
- **hub → node** confirms a spontaneous push (an IS with FLAGS.PUSH=1, §8.3) was
  received, so the node can stop retransmitting it (§9).

---

## 8. Transactions

### 8.1 Read

```
Hub  →  Node    TYPE=GET    REG=R  VALUE=0
Node →  Hub     TYPE=IS     REG=R  VALUE=<current value>
```

### 8.2 Write

```
Hub  →  Node    TYPE=SET    REG=R  VALUE=<requested value>
Node →  Hub     TYPE=ACK    REG=R  VALUE=0
```

The node replies with an ACK to confirm receipt of the write; the ACK carries no value. A write may be applied asynchronously (it can take time, or be clamped or rejected for a read-only register), so the node does not report the resulting value inline. The hub learns the true state from a WATCH push (§8.3) or a subsequent GET (§8.1).

A SET with FLAGS.NULL=1 clears the register: VALUE is undefined and the hub is asking the node to unset it (the dual of a NULL IS, §7). The node still replies with an ACK. A node that has no notion of an unset register ignores the NULL write.

### 8.3 Subscribe

```
Hub  →  Node    TYPE=WATCH  REG=R  VALUE=1
Node →  Hub     TYPE=IS     REG=R  VALUE=<current value>            (immediate reply)
Node →  Hub     TYPE=IS     REG=R  VALUE=<new value>  FLAGS.PUSH=1  (on each change)
Hub  →  Node    TYPE=ACK    REG=R  VALUE=0                          (acknowledges the push)
```

The immediate reply is solicited — it answers the WATCH — so it is a plain IS
with PUSH clear, recovered by the hub's request retransmission like any other
reply (§9). Every subsequent change notification is unsolicited: the node sets
FLAGS.PUSH=1 and the hub returns an ACK for it. Until that ACK arrives the node
retransmits the push, so a change is not lost when a push collides with hub
traffic (§9).

#### Watch-all (REG=0)

A WATCH for the reserved register `REG=0` subscribes to (or unsubscribes from)
**every** register of the node at once. Tags are non-zero by construction (§11),
so `REG=0` is free as this all-registers sentinel.

```
Hub  →  Node    TYPE=WATCH  REG=0  VALUE=1                          (watch-all)
Node →  Hub     TYPE=ACK    REG=0  VALUE=0                          (single reply)
Node →  Hub     TYPE=IS     REG=R  VALUE=<new value>  FLAGS.PUSH=1  (on each change)
Hub  →  Node    TYPE=ACK    REG=R  VALUE=0                          (acknowledges the push)
Hub  →  Node    TYPE=WATCH  REG=0  VALUE=0                          (unwatch-all)
```

Unlike a single-register WATCH, the node does **not** dump current values: it
answers a watch-all with one ACK (`REG=0`) and nothing else. The hub seeds the
initial values it needs with GETs (§8.1), retrying a register's GET until the
node answers (a watch-all refresh, unlike a single-register one, draws no value
to seed from). The hub re-seeds the same way whenever a register loses its value
— in particular after the node is reported `NULL` and later answers again (e.g.
a radio link that dropped and recovered): a recovered watch-all refresh restores
only liveness, so the hub re-GETs each register to repopulate it rather than
waiting for its next change. Thereafter the node pushes each
changed register exactly as for an individual subscription — a real `REG`, with
FLAGS.PUSH=1, acknowledged per push. A node sends one push per change even when a
hub holds both a watch-all and an individual watch for that register.

Watch-all collapses what would otherwise be one WATCH refresh per register into a
single refresh per node (§10), which both saves airtime and avoids overflowing a
node's bounded subscription table when a hub watches more registers than the
table holds. Because the whole node is one subscription, liveness (§10) is
per-node: when a watch-all node stops answering refreshes, the hub reports all of
its registers as `NULL` together.

### 8.4 Unsubscribe

```
Hub  →  Node    TYPE=WATCH  REG=R  VALUE=0
Node →  Hub     TYPE=IS     REG=R  VALUE=<current value>
```

---

## 9. Reliability

The protocol is **best-effort at the RF layer**. Reliability is the hub's responsibility:

- After sending a request the hub waits up to `T_timeout` (recommended: 50 ms) for a matching response (`SRC == expected node`, `REG == sent REG`, and the expected reply TYPE: ACK for a SET, IS for a GET/WATCH).
- The hub asks the node to defer its reply by `GUARD` milliseconds (§6), chosen from the hub radio's transmit-to-receive turnaround time, so the radio is listening again before the reply arrives. `GUARD` is always smaller than `T_timeout` — a hub that cannot honour this (its radio's guard would not leave room for a reply under the timeout) must refuse to start rather than lose every reply.
- If no response arrives within `T_timeout`, the hub may retransmit the same request up to `N_retry` times (recommended: 3).

Solicited replies are made reliable by the hub retransmitting the request, but an
unsolicited push (§8.3) has no outstanding request to retransmit, and the node is
half-duplex and blind to the hub's transmit windows — a push can collide with hub
traffic and be lost with nothing to recover it. Pushes therefore carry their own
acknowledgement:

- A node marks every spontaneous change notification with FLAGS.PUSH=1 (§6) and
  keeps retransmitting it until the hub acknowledges it.
- The hub replies to a received push with an `ACK` for the same `REG` (VALUE=0).
  The ACK is itself best-effort; a duplicate push (because its ACK was lost)
  carries the same value and is simply re-acknowledged, so loss of an ACK is
  harmless.
- The node retransmits a pending push every `T_push` (the reference firmware
  uses 60 ms) up to `N_push` times (reference: 16) before giving up. A newer
  value for the same register supersedes any still-pending push for it.

---

## 10. Push Subscription Lifecycle

- A node keeps at most one subscription per register per hub.
- A subscription expires if the node receives no packet from that hub for `T_idle` (recommended: 60 s). After expiry, push stops silently.
- To keep subscriptions alive, the hub periodically re-sends `WATCH` for every active subscription well within `T_idle` (the reference host hub uses a 15 s refresh interval). Re-`WATCH` also re-establishes any subscription a node may have dropped (e.g. after a reboot).
- The refresh doubles as a liveness check: each re-`WATCH` draws an immediate `IS` reply from a live node. When a node stops answering refreshes (e.g. it loses power), the hub treats the register as having no value after a few consecutive misses and reports it as `NULL`, so a vanished node's last value is not served indefinitely. The next successful refresh (or any push) restores the real value. The reference host hub marks a node offline after 2 missed refreshes (~30 s).
- Change detection is implementation-defined. The hub may fall back to polling if a node pushes too frequently.

---

## 11. Register Model and Provisioning

The protocol carries registers as bare `uint16` IDs on the wire (§4). Names,
types, scaling, and metadata are **hub-side knowledge** and are deliberately
absent from the wire format for performance and memory reasons. The mapping
between wire IDs and named, typed registers lives in the host as
**inventory-as-code**: a Go program that the host library compiles and runs.
There is no runtime descriptor exchange and no offline code-generation step.

This section defines the register model (tags, types, scaling), the
inventory-as-code authoring model (device types and instances), and the
node-identity provisioning path.

### 11.1 Concepts

| Concept | Role | Source of truth | Carries wire IDs? |
|---------|------|-----------------|-------------------|
| **Device type** | A reusable, named register table (`Name` + `Registers`). Defines register *names*, tags, types, scaling, and metadata. | Hand-authored Go (device-type module's `Type()`) | Yes (tags) |
| **Instance** | One physical device: its name, MCU `UID`, key, channel, device type, and config. Describes *what a node is and where it is*. | Hand-authored Go (the site inventory) | n/a |
| **Node identity** | The node's `address` (§3) and shared `key` (§5). Per-chip; the key is secret. | Provisioning page (written over SWD) | n/a |

The single most important rule: **a register's wire identity is its hand-assigned
`Tag`** (a `uint8`, like a protobuf field number) — unique and non-zero within a
device type, and **never reused** once retired. Because tags are stable by
construction, firmware and host cannot drift: reordering or extending a register
table never changes any existing register's wire ID.

```
device type (Go)                ──┐
 (registers: tag, name, scaling)  │
                                  ├─► host runtime ──► Registry bridge
instance (Go)                     │
 (UID, key, channel, config) ─────┘
```

The wire format (§4) and the node-side protocol logic operate on a flat `uint16`
REG. The host maps each register's `Tag` to the `uint16` carried on the wire
(`REG = Tag`); the firmware likewise knows its registers by tag.

### 11.2 Device Type

A device type is a reusable register table, authored as Go source in the
device-type module and returned by its `Type()` function. Register names and
tags are unique within the type.

| Field      | Type                 | Description                                                  |
|------------|----------------------|-------------------------------------------------------------|
| name       | string               | Device-type name (e.g. `bob`)                               |
| registers  | list<Register>       | Register table (see §11.3)                                  |

### 11.3 Register

| Field      | Type               | Description                                                   |
|------------|--------------------|---------------------------------------------------------------|
| tag        | uint8              | Permanent wire identity: unique, non-zero, never reused       |
| name       | string             | Register name, unique within the device type                  |
| type       | enum               | `int`, `float`, or `bool`                                     |
| multiplier | int32              | Hub scaling: `display = wire × multiplier / divider`          |
| divider    | int32              | See multiplier; must not be zero for non-`bool` registers     |
| metadata   | map<string,string> | Key-value pairs merged into the hub's register record         |

All registers carry `int32` on the wire (§8). `type`, `multiplier`, and
`divider` are hub-side hints only — the node always sends raw `int32`.

- **`int`** — no scaling; multiplier=1, divider=1 is the identity.
- **`float`** — wire value is scaled: e.g. multiplier=1, divider=100 means wire value `1234` displays as `12.34`.
- **`bool`** — wire value 0 = false, 1 = true; multiplier/divider are ignored.

The register name is scoped by the device instance name when published to the
Registry: instance `kitchen` + register `temperature` → `kitchen.temperature`.
This keeps names distinct across devices that share one device type.

### 11.4 Inventory

A deployment is an `Inventory`: a list of `Instance`s, each binding one physical
device to its identity and type. A device type may be instantiated any number of
times.

**Instance**

| Field    | Type      | Description                                                       |
|----------|-----------|------------------------------------------------------------------|
| name     | string    | Device name, unique within the inventory (scopes register names) |
| uid      | [12]byte  | MCU unique ID; the RF address is derived from it (§11.5)         |
| key      | [16]byte  | XTEA shared key (§5)                                             |
| channel  | uint8     | RF channel the device listens and transmits on (§2)             |
| type     | DeviceType| The device's register table (§11.2)                             |
| config   | any       | Device-type-specific configuration (a fixed-size struct)        |

The host validates the inventory at startup: tags are unique and non-zero within
each device type, and instance names are unique.

### 11.5 Node Identity Provisioning

`address` (§3) and `key` (§5) are **not** part of any device type. They are
per-chip and the key is secret, so they are written to the device by the
provisioning tooling over SWD, together with the device's RF channel and config,
as a single **provisioning page** in flash.

The RF address is **derived, not stored**: both the host and the firmware compute
`address = CRC32(MCU_UID)` (§3), so it never appears in the inventory. The host
reads the device's 12-byte UID over SWD, looks the device up in the inventory by
UID, and writes its page.

The **provisioning page** layout (encoded identically by host and firmware, so
they cannot disagree on the bytes):

```
header  magic | layout | configLen | channel | pad | address | key
config  the device type's fixed-size config struct
crc32   CRC-32 (IEEE) over everything before it
```

The firmware reads the page once at boot to learn its identity and config; it
emits **no descriptor at runtime**. A device whose page has never been written is
detected by a magic mismatch (an erased page), distinct from a corrupt page
(CRC mismatch).

### 11.6 Worked Example

A `bob` device type, authored in Go (tags hand-assigned, permanent):

```go
inventory.DeviceType{
    Name: "bob",
    Registers: []inventory.Register{
        {Tag: 1, Name: "green", Type: inventory.TypeInt, Multiplier: 1, Divider: 1},
        {Tag: 2, Name: "red", Type: inventory.TypeInt, Multiplier: 1, Divider: 1},
        {Tag: 3, Name: "gpio", Type: inventory.TypeInt, Multiplier: 1, Divider: 1},
    },
}
```

A site inventory instantiates it per physical device:

```go
inventory.Inventory{
    {
        Name:    "bob",
        UID:     [12]byte{ /* MCU unique ID, read over SWD */ },
        Key:     [16]byte{ /* XTEA key */ },
        Channel: 37,
        Type:    bob.Type(),
        Config:  bob.Config{DefaultRedPeriod: 500, DefaultGreenPeriod: 100},
    },
}
```

The host derives `bob`'s address as `CRC32(UID)`, maps each register's tag to
its wire REG, and publishes `bob.green`, `bob.red` and `bob.gpio` to the
Registry. Two instances of the same type coexist because their *names* differ;
the wire never sees the instance concept.

### 11.7 Onboarding and Provisioning Workflow

1. **Onboard.** Attach a new device and run `new`: the host reads its UID over
   SWD and prints a paste-ready `Instance{}` stub. Fill in the name, key,
   channel, type and config, and commit it to the inventory source.
2. **Provision.** Run `provision`: the host reads the attached device's UID,
   finds the matching inventory instance **by UID alone**, builds its
   provisioning page (address = `CRC32(UID)`, key, channel, config) and writes it
   to flash over SWD.
3. **Run.** Run `hub`: the host builds every inventory device's register
   descriptor, derives its address, and bridges its registers to the Registry.

Adding or provisioning a device is an edit to the inventory **source**, type-checked
by the Go compiler — there are no JSON files to hand-edit and no descriptor pool
to keep in sync.

---

## 12. Radio Interface (implementation contract)

Any radio backend must provide these three operations to the protocol layer. All operations are **non-blocking** from the protocol layer's perspective:

```
Send(dst [4]byte, payload []byte) error
    Set the RF sync word to dst (the destination device address),
    then transmit one packet. Blocks only for the duration of the
    air transmission itself (~0.5 ms at 250 kbps). Returns
    immediately after the TX-complete interrupt; does NOT wait for
    a response.

Receive(buf []byte) (n int, ok bool)
    If a packet is waiting in the RX FIFO, copy it into buf and
    return (n, true). Otherwise return (0, false) immediately
    without blocking.
```

The protocol layer must not assume any underlying transport beyond these three operations.
