# BleRiot Protocol Specification

Hardware-independent specification for the BleRiot IoT register protocol.

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

```
Bit 7–1  — reserved, must be 0
Bit 0    — NULL: VALUE field is absent; register has no value
```

When NULL=1 the VALUE field is undefined and must be ignored by the receiver.

---

## 7. TYPE Byte

| Value | Name  | Sender | Description                                        |
|-------|-------|--------|----------------------------------------------------|
| 0x00  | GET   | hub    | Read current register value (one-shot)             |
| 0x01  | SET   | hub    | Write register value                               |
| 0x02  | IS    | node   | Current register value (reply to GET/WATCH)        |
| 0x03  | WATCH | hub    | Subscribe (VALUE=1) or unsubscribe (VALUE=0)       |
| 0x04  | ACK   | node   | Acknowledges a SET; carries no value               |

Receivers must ignore packets with unknown TYPE values.

A node sends IS with FLAGS.NULL=1 when a register has no value (e.g. sensor not yet ready, hardware fault, or explicitly unset).

An ACK confirms only that a SET was received. It carries no value (VALUE=0, FLAGS=0): a write may be applied asynchronously, so the node does not report a result inline. The hub observes the resulting value through a WATCH subscription or a subsequent GET.

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
Node →  Hub     TYPE=IS     REG=R  VALUE=<current value>   (immediate reply)
Node →  Hub     TYPE=IS     REG=R  VALUE=<new value>       (on each change)
```

### 8.4 Unsubscribe

```
Hub  →  Node    TYPE=WATCH  REG=R  VALUE=0
Node →  Hub     TYPE=IS     REG=R  VALUE=<current value>
```

---

## 9. Reliability

The protocol is **best-effort at the RF layer**. Reliability is the hub's responsibility:

- After sending a request the hub waits up to `T_timeout` (recommended: 50 ms) for a matching response (`SRC == expected node`, `REG == sent REG`, and the expected reply TYPE: ACK for a SET, IS for a GET/WATCH).
- If no response arrives within `T_timeout`, the hub may retransmit the same request up to `N_retry` times (recommended: 3).

---

## 10. Push Subscription Lifecycle

- A node keeps at most one subscription per register per hub.
- A subscription expires if the node receives no packet from that hub for `T_idle` (recommended: 60 s). After expiry, push stops silently.
- To keep subscriptions alive, the hub periodically re-sends `WATCH` for every active subscription well within `T_idle` (the reference host hub uses a 15 s refresh interval). Re-`WATCH` also re-establishes any subscription a node may have dropped (e.g. after a reboot).
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
| name       | string               | Device-type name (e.g. `thermostat`)                        |
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

A `thermostat` device type, authored in Go (tags hand-assigned, permanent):

```go
inventory.DeviceType{
    Name: "thermostat",
    Registers: []inventory.Register{
        {Tag: 1, Name: "temperature", Type: inventory.TypeFloat, Multiplier: 1, Divider: 100,
            Metadata: map[string]string{"unit": "celsius"}},
        {Tag: 2, Name: "setpoint", Type: inventory.TypeFloat, Multiplier: 1, Divider: 100,
            Metadata: map[string]string{"unit": "celsius"}},
        {Tag: 3, Name: "heating", Type: inventory.TypeBool},
    },
}
```

A site inventory instantiates it per physical device:

```go
inventory.Inventory{
    {
        Name:    "kitchen",
        UID:     [12]byte{ /* MCU unique ID, read over SWD */ },
        Key:     [16]byte{ /* XTEA key */ },
        Channel: 37,
        Type:    thermostat.Type(),
        Config:  thermostat.Config{MinTemp: 18, MaxTemp: 22},
    },
}
```

The host derives `kitchen`'s address as `CRC32(UID)`, maps each register's tag to
its wire REG, and publishes `kitchen.temperature`, `kitchen.setpoint` and
`kitchen.heating` to the Registry. Two instances of the same type coexist because
their *names* differ; the wire never sees the instance concept.

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
