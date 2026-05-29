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
| PDU size         | 20 bytes (fixed)                                                        |

Each node operates on a single channel defined in its provisioning descriptor. The hub may have multiple radio interfaces, each assigned to a different channel, allowing nodes to be grouped by channel for spectrum spread or logical partitioning.

The RF sync word for each transmission is set to the 4-byte destination device address. Radio hardware that supports address/pipe filtering must configure its receive address to the device's own address, so only packets destined for that device are passed to the protocol layer. The 32-bit address space makes accidental collision with foreign RF traffic negligible.

---

## 3. Device Addressing

Each device has a **4-byte address** derived from a hardware unique identifier (e.g. CRC32 of the MCU UID, or CRC32 of a host MAC address). Addresses are treated as opaque 32-bit values.

Reserved address: `0x00000000` — must not be assigned to any device.

---

## 4. Packet Format

All packets share the same fixed 20-byte structure:

```
Offset  Size  Field
──────  ────  ─────────────────────────────────────────────────
0       4     SRC   — source device address (little-endian, plaintext)
4       16    BLOCK — AES-128-ECB encrypted block (see §5):
                        TYPE  (1 byte)  — packet type (see §6)
                        FLAGS (1 byte)  — options (see §7)
                        REG   (2 bytes) — register address (uint16, little-endian)
                        VALUE (4 bytes) — int32, little-endian (zero in GET/WATCH)
                        NONCE (8 bytes) — random, prevents ECB ciphertext reuse
──────  ────
Total: 20 bytes
```

The destination is not carried in the payload. It is encoded as the RF sync word (§2), which the receiver's hardware uses for filtering. A received packet is therefore always addressed to the receiving device.

SRC is plaintext so the receiver can look up the sender's AES key before decrypting BLOCK.

All multi-byte fields inside BLOCK are **little-endian**.

---

## 5. Security

All packets are encrypted with **AES-128-ECB** using the node's shared key (provisioned via §10). The 16-byte BLOCK field contains the payload and an 8-byte random NONCE. The NONCE ensures that repeated identical payloads produce different ciphertexts.

The hub decrypts each received packet using the AES key associated with the SRC address. Packets from unknown addresses are silently discarded.

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
- Change detection is implementation-defined. The hub may fall back to polling if a node pushes too frequently.

---

## 11. Provisioning

Node provisioning is a one-time, one-way transfer that delivers a **node descriptor** to the hub over a side-channel (SWD/RTT, UART, IrDa, or similar). The descriptor is encoded as YAML.

### 11.1 Framing

The descriptor is delimited by a sentinel line and a trailing empty line, so the hub can extract it from a noisy output stream (e.g. mixed RTT debug output):

```
--- bleriot-node
address: 0xA3F2B841
key: 9f3c1e8a2b7d4f06e5a0c3d1b8f92e47
...

```

- **Start sentinel:** a line equal to `--- bleriot-node`
- **End:** the first empty line after the sentinel
- The captured block (sentinel line through the last non-empty line) is valid YAML and can be fed directly to any YAML parser.

### 11.2 Node Descriptor

| Field      | Type               | Description                                                   |
|------------|--------------------|---------------------------------------------------------------|
| address    | uint32             | Node RF address (§3), hex-encoded (e.g. `0xA3F2B841`)        |
| channel    | uint8              | RF channel the node listens and transmits on                  |
| key        | bytes[16]          | AES-128 shared secret, hex-encoded (32 hex chars)             |
| metadata   | map<string,string> | Key-value pairs merged into the hub's node record             |
| registers  | list<Register>     | Register descriptors (see §11.3)                              |

### 11.3 Register Descriptor

| Field      | Type               | Description                                                   |
|------------|--------------------|---------------------------------------------------------------|
| id         | uint16             | Register address used in RF packets (REG field, §4)           |
| name       | string             | Human-readable name (e.g. `temperature`)                      |
| type       | enum               | `int`, `float`, or `bool`                                     |
| multiplier | int32              | Hub scaling: `display = wire × multiplier / divider`          |
| divider    | int32              | See multiplier; must not be zero                              |
| metadata   | map<string,string> | Key-value pairs merged into the hub's register record         |

All registers carry `int32` on the wire (§8). `type`, `multiplier`, and `divider` are hub-side hints only — the node always sends raw `int32`.

- **`int`** — no scaling; multiplier=1, divider=1 is the identity.
- **`float`** — wire value is scaled: e.g. multiplier=1, divider=100 means wire value `1234` displays as `12.34`.
- **`bool`** — wire value 0 = false, 1 = true; multiplier/divider are ignored.

### 11.4 Example

```yaml
--- bleriot-node
address: 0xA3F2B841
channel: 10
key: 9f3c1e8a2b7d4f06e5a0c3d1b8f92e47
metadata:
  location: garage
  hw_rev: "1.3"
registers:
  - id: 0x0001
    name: temperature
    type: float
    multiplier: 1
    divider: 100
    metadata:
      unit: celsius
  - id: 0x0002
    name: relay
    type: bool

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
