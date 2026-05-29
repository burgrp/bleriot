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

Each node operates on a single channel defined in its provisioning descriptor. The hub may have multiple radio interfaces, each assigned to a different channel, allowing nodes to be grouped by channel for spectrum spread or logical partitioning.

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
| 0x02  | IS    | node   | Current register value (reply to any hub packet)   |
| 0x03  | WATCH | hub    | Subscribe (VALUE=1) or unsubscribe (VALUE=0)       |

Receivers must ignore packets with unknown TYPE values.

A node sends IS with FLAGS.NULL=1 when a register has no value (e.g. sensor not yet ready, hardware fault, or explicitly unset).

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
Node →  Hub     TYPE=IS     REG=R  VALUE=<actual value>
```

The node always responds with the *actual* register value after applying the write. If the register is read-only or the value was clamped, the hub learns the true state from the response.

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

- After sending a request the hub waits up to `T_timeout` (recommended: 50 ms) for a matching response (`SRC == expected node`, `REG == sent REG`).
- If no response arrives within `T_timeout`, the hub may retransmit the same request up to `N_retry` times (recommended: 3).

---

## 10. Push Subscription Lifecycle

- A node keeps at most one subscription per register per hub.
- A subscription expires if the node receives no packet from that hub for `T_idle` (recommended: 60 s). After expiry, push stops silently.
- To keep subscriptions alive, the hub periodically re-sends `WATCH` for every active subscription well within `T_idle` (the reference host hub uses a 15 s refresh interval). Re-`WATCH` also re-establishes any subscription a node may have dropped (e.g. after a reboot).
- Change detection is implementation-defined. The hub may fall back to polling if a node pushes too frequently.

---

## 11. Register Model, Descriptors, and Code Generation

The protocol carries registers as bare `uint16` IDs on the wire (§4). Names,
types, scaling, and metadata are **hub-side knowledge** and are deliberately
absent from the wire format for performance and memory reasons. The mapping
between wire IDs and named, typed registers is produced by an offline
**code-generation** step, not transmitted by the node at runtime.

This section defines the authoring model (class descriptors, node specs), the
generated artifacts (node code, node descriptor), the deterministic ID
allocation algorithm, and the separate node-identity provisioning path.

### 11.1 Concepts

| Concept | Role | Source of truth | Carries wire IDs? |
|---------|------|-----------------|-------------------|
| **Class descriptor** | A reusable, named set of registers (a "register profile"). Defines register *names*, types, scaling, and metadata only. | Hand-authored library | No |
| **Node spec** | Composition of one or more class *instances*, plus channel and node metadata. Describes *what a node is*. | Hand-authored, per node-type | No |
| **Generated node code** | Per-node firmware artifact: `const` wire IDs, one interface per class, and a wiring table. | Code generator | Yes (assigned) |
| **Generated node descriptor** | Per-node hub artifact: the resolved flat list of `id → {name, type, scaling, metadata}`. | Code generator | Yes (assigned) |
| **Node identity** | The node's `address` (§3) and shared `key` (§5). Per-chip, secret. | Runtime / provisioning tool | n/a |

The single most important rule: **the code generator is the sole authority that
assigns wire IDs.** Class descriptors and node specs never contain IDs. Both
generated artifacts are produced from the same generator run, so the firmware
and the hub descriptor cannot drift.

```
class descriptors          ──┐
 (names only, NO wire IDs)   │
                             ├─► codegen ──┬─► node code (const IDs + interfaces + wiring)
node spec                    │             │
 (which classes/instances) ──┘             └─► node descriptor (resolved id→name/meta)
                                                      │
                                                      └─► consumed by hub
```

The wire format (§4) and the node-side protocol logic are **unchanged** by this
model — they still operate on a flat `uint16` REG. All class/instance/offset
resolution happens at generation time on the host, never on the MCU.

### 11.2 Class Descriptor

A class descriptor is a reusable register profile. It is authored as Go source
(values consumed by the generator). Register names are unique within the class.

| Field      | Type                 | Description                                                  |
|------------|----------------------|-------------------------------------------------------------|
| name       | string               | Class name (e.g. `thermometer`), unique within the library  |
| registers  | list<Register>       | Register descriptors (see §11.3)                            |
| metadata   | map<string,string>   | Key-value pairs merged into every instance's hub record     |

### 11.3 Register Descriptor

Note: there is **no `id` field**. The wire ID is assigned by the generator
(§11.6).

| Field      | Type               | Description                                                   |
|------------|--------------------|---------------------------------------------------------------|
| name       | string             | Register name, unique within its class (e.g. `temperature`)   |
| type       | enum               | `int`, `float`, or `bool`                                     |
| multiplier | int32              | Hub scaling: `display = wire × multiplier / divider`          |
| divider    | int32              | See multiplier; must not be zero                              |
| metadata   | map<string,string> | Key-value pairs merged into the hub's register record         |

All registers carry `int32` on the wire (§8). `type`, `multiplier`, and
`divider` are hub-side hints only — the node always sends raw `int32`.

- **`int`** — no scaling; multiplier=1, divider=1 is the identity.
- **`float`** — wire value is scaled: e.g. multiplier=1, divider=100 means wire value `1234` displays as `12.34`.
- **`bool`** — wire value 0 = false, 1 = true; multiplier/divider are ignored.

### 11.4 Node Spec

A node spec composes class instances into a concrete node-type. A class may be
instantiated more than once; each instance has a name unique within the node.

| Field      | Type                 | Description                                                       |
|------------|----------------------|------------------------------------------------------------------|
| name       | string               | Node-type name (used to name generated artifacts)                |
| channel    | uint8                | RF channel the node listens and transmits on (§2)                |
| metadata   | map<string,string>   | Key-value pairs merged into the hub's node record                |
| instances  | list<ClassInstance>  | Class instances composed onto this node (see below)              |

**ClassInstance**

| Field    | Type   | Description                                                          |
|----------|--------|---------------------------------------------------------------------|
| class    | string | Name of a class descriptor (§11.2)                                  |
| name     | string | Instance name, unique within the node (e.g. `outdoor`, `relay_a`)  |

The **qualified register name** is `instanceName + "." + registerName`
(e.g. `outdoor.temperature`). Qualified names are unique within a node and are
the keys used by both ID allocation (§11.6) and the hub.

### 11.5 Node Identity Provisioning

`address` (§3) and `key` (§5) are **not** part of any descriptor file. They are
per-chip and the key is secret, so they are delivered to the hub by the
provisioning tooling out of band — for example, read/derived over SWD at flash
time (`address = CRC32(MCU_UID)`), with the key injected during the same step.

The node firmware therefore emits **no descriptor at runtime**. The hub obtains
the generated node descriptor (§11.7) directly as a file and merges in the
identity record produced by the provisioning tool.

### 11.6 Wire ID Allocation (generator algorithm)

The generator assigns a `uint16` to every qualified register name
deterministically:

1. **Collect** all qualified names across every instance in the node spec.
2. **Canonical order:** sort qualified names lexicographically. Allocation is
   performed in this order so the result is independent of the authoring order
   of classes or instances in the node spec.
3. **Primary slot:** `id = fnv1a32(qualifiedName) & 0xFFFF`. The reserved value
   `0x0000` is never assigned; if the hash yields `0x0000`, treat it as a
   collision.
4. **Collision resolution:** if the slot is already taken (or reserved), linear
   probe `(id + 1) & 0xFFFF`, skipping `0x0000`, until a free slot is found.
5. **Version hash:** the generator computes a descriptor version hash over the
   full resolved set of `(id, qualifiedName, type, multiplier, divider)` tuples
   in canonical order. This hash is embedded in both the generated node code and
   the generated node descriptor so the hub can detect a firmware/descriptor
   mismatch.

> **Stability note.** Hash-based allocation is stable across reordering the node
> spec. Adding a register can still shift the IDs of later-probed entries that
> collide with the newcomer's slot. The version hash makes such a shift
> detectable; pinning IDs across firmware versions (e.g. via a checked-in
> lockfile) is a possible future hardening and is out of scope here.

### 11.7 Generated Artifacts

Both artifacts are produced by one generator run and are **checked into version
control**.

**Node code** (Go, compiled into firmware) provides, per node:

- A `const` for each qualified register's wire ID, e.g.
  `RegOutdoorTemperature uint16 = 0x1A3F`.
- A slice of all wire IDs (`RegisterIDs`) for iteration / table setup.
- The descriptor version hash as a `const`.

No per-class wrappers or interfaces are generated. The firmware backs registers
generically, keyed by `uint16` wire ID; classes and instances exist only at
generation time. No register names, types, scaling, or metadata appear in
firmware — only IDs. This keeps flash/RAM cost minimal.

**Node descriptor** (consumed by the hub) is the resolved flat list. It is
emitted as a data file (JSON) so the hub need not import generator packages.
The hub is a generic bridge: it reads this descriptor and maps every BleRiot
register to a register in the external Registry service, without any
class-specific logic.

The descriptor is a **shared, per-type** artifact and carries **no node name**.
A node's name and its provisioned identity live in a separate per-device
instance file on the hub (§11.9), so one descriptor can back many physical
devices.

```json
{
  "channel": 10,
  "version": "0x9F3C1E8A",
  "metadata": { "hw_rev": "1.3" },
  "registers": [
    {
      "id": 6719,
      "name": "outdoor.temperature",
      "class": "thermometer",
      "instance": "outdoor",
      "type": "float",
      "multiplier": 1,
      "divider": 100,
      "metadata": { "unit": "celsius" }
    },
    {
      "id": 2048,
      "name": "main.relay",
      "class": "switch",
      "instance": "main",
      "type": "bool",
      "multiplier": 1,
      "divider": 1,
      "metadata": {}
    }
  ]
}
```

### 11.8 Worked Example

Authored classes (names only, no IDs):

```
class "thermometer":
  registers:
    - name: temperature   type: float   multiplier: 1  divider: 100  metadata: {unit: celsius}
    - name: humidity       type: int      multiplier: 1  divider: 1

class "switch":
  registers:
    - name: relay          type: bool
```

Authored node spec (`garage-controller`, channel 10):

```
instances:
  - class: thermometer  name: outdoor
  - class: switch        name: main
  - class: switch        name: aux
```

The generator collects qualified names `aux.relay`, `main.relay`,
`outdoor.humidity`, `outdoor.temperature`, allocates a `uint16` to each per
§11.6, then emits the node code (const IDs + `Thermometer` and `Switch`
interfaces + wiring table) and the JSON node descriptor above. Two `switch`
instances coexist because their qualified names differ — the wire never sees the
instance concept.

### 11.9 Hub Node Files

The hub does not list nodes in its main config. Instead the config names a
**nodes directory** (`nodesDir`), and every `*.json` file in it is one physical
node. This keeps provisioning a new device to a single file drop — the hub
config is never edited.

A node file is a thin **instance file**: it references a shared descriptor
(§11.7) and carries the device's provisioned identity (§11.5). The file's base
name is the node name (so the descriptor itself needs no name field). The
`descriptor` path is resolved relative to the node file's own directory.

```
hub.json                     # nodesDir: "nodes"
descriptors/
  thermo.json                # shared per-type descriptor (generated, §11.7)
nodes/
  outdoor.json               # node "outdoor"
  garage.json                # node "garage"  (same descriptor, different identity)
```

```json
// nodes/outdoor.json
{
  "descriptor": "../descriptors/thermo.json",
  "address": "CCA00002",
  "key": "00112233445566778899AABBCCDDEEFF"
}
```

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
