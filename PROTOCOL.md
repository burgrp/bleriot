# BleRiot Protocol Specification

Hardware-independent specification for the BleRiot IoT register protocol.

---

## 1. Overview

BleRiot is a simple request/response protocol for reading and writing named integer registers on IoT nodes. A single hub polls one or more nodes. Nodes never initiate communication unless explicitly subscribed.

- **Topology:** star — one hub, many nodes
- **Initiator:** hub only (except subscribed push packets)
- **Register type:** `int32` (signed 32-bit integer)

---

## 2. RF Physical Layer

These parameters are mandatory for all BleRiot-compatible radio implementations:

| Parameter        | Value                                                                   |
|------------------|-------------------------------------------------------------------------|
| Frequency        | 2440 MHz                                                                |
| Sync word        | Destination device address (4 bytes, little-endian) — see §3           |
| Data rate        | 250 kbps                                                                |
| Modulation       | GFSK                                                                    |
| Packet format    | BLE-compatible (preamble, sync word, PDU, 3-byte CRC, whitening)       |
| PDU size         | 12 bytes (fixed)                                                        |

The RF sync word for each transmission is set to the 4-byte destination device address. Radio hardware that supports address/pipe filtering must configure its receive address to the device's own address, so only packets destined for that device are passed to the protocol layer. The 32-bit address space makes accidental collision with foreign RF traffic negligible.

---

## 3. Device Addressing

Each device has a **4-byte address** derived from a hardware unique identifier (e.g. CRC32 of the MCU UID, or CRC32 of a host MAC address). Addresses are treated as opaque 32-bit values.

Reserved address: `0x00000000` — must not be assigned to any device.

---

## 4. Packet Format

All packets share the same fixed 12-byte structure:

```
Offset  Size  Field
──────  ────  ─────────────────────────────────────────────────
0       4     SRC   — source device address (little-endian)
4       1     FLAGS — operation and options (see §5)
5       1     SEQ   — sequence number
6       2     REG   — register address (little-endian, uint16)
8       4     VALUE — int32, little-endian (zero in read requests)
──────  ────
Total: 12 bytes
```

The destination is not carried in the payload. It is encoded as the RF sync word (§2), which the receiver's hardware uses for filtering. A received packet is therefore always addressed to the receiving device.

All multi-byte fields are **little-endian**.

---

## 5. FLAGS Byte

```
Bit 7     — reserved, must be 0
Bit 6     — reserved, must be 0
Bit 5     — reserved, must be 0
Bit 4     — reserved, must be 0
Bit 3     — reserved, must be 0
Bit 2     — PUSH:  0 = one-shot,  1 = subscribe to changes
Bit 1     — DIR:   0 = request,   1 = response
Bit 0     — OP:    0 = read,      1 = write
```

Defined FLAGS combinations:

| Name            | FLAGS | Description                                      |
|-----------------|-------|--------------------------------------------------|
| READ_REQ        | 0x00  | Hub reads a register (one-shot)                  |
| READ_REQ_PUSH   | 0x04  | Hub reads + subscribes to changes                |
| WRITE_REQ       | 0x01  | Hub writes a register                            |
| READ_RESP       | 0x02  | Node replies to read (or push notification)      |
| WRITE_RESP      | 0x03  | Node confirms write (echoes actual value)        |
| PUSH            | 0x06  | Node unsolicited push (SEQ = 0xFF)               |

Receivers must ignore packets with unknown FLAGS values.

---

## 6. Transactions

### 6.1 Read (one-shot)

```
Hub  →  Node    FLAGS=0x00  SEQ=N  REG=R  VALUE=0
Node →  Hub     FLAGS=0x02  SEQ=N  REG=R  VALUE=<current value>
```

### 6.2 Write

```
Hub  →  Node    FLAGS=0x01  SEQ=N  REG=R  VALUE=<requested value>
Node →  Hub     FLAGS=0x03  SEQ=N  REG=R  VALUE=<actual value>
```

The node always responds with the *actual* register value after applying the write. If the register is read-only, or the value was clamped, the hub learns the true state from the response.

### 6.3 Subscribe

```
Hub  →  Node    FLAGS=0x04  SEQ=N  REG=R  VALUE=0   (subscribe)
Node →  Hub     FLAGS=0x02  SEQ=N  REG=R  VALUE=<current value>  (immediate reply)
Node →  Hub     FLAGS=0x06  SEQ=0xFF REG=R VALUE=<new value>     (on change, repeated)
```

### 6.4 Unsubscribe

```
Hub  →  Node    FLAGS=0x00  SEQ=N  REG=R  VALUE=0   (plain read, PUSH=0)
Node →  Hub     FLAGS=0x02  SEQ=N  REG=R  VALUE=<current value>
```

Sending a one-shot read for a subscribed register cancels the subscription.

---

## 7. Sequence Numbers

- The hub maintains a per-node SEQ counter, incrementing with each new request (wraps 0x00–0xFE).
- The node echoes SEQ from the request into its response.
- `0xFF` is reserved for unsolicited PUSH packets and must not be used by the hub.
- The hub uses SEQ to correlate responses with pending requests and to discard stale duplicates from retries.

---

## 8. Reliability

The protocol is **best-effort at the RF layer**. Reliability is the hub's responsibility:

- After sending a request the hub waits up to `T_timeout` (recommended: 50 ms) for a matching response (`SRC == expected node`, `SEQ == sent SEQ`).
- If no response arrives within `T_timeout`, the hub may retransmit the same request (same SEQ) up to `N_retry` times (recommended: 3).
- The node ignores duplicate requests (same SEQ from same SRC) while its response is still pending. Once a response has been sent, a repeated request is processed again.

---

## 9. Push Subscription Lifecycle

- A node keeps at most one subscription per register per hub.
- A subscription expires if the node receives no packet from that hub for `T_idle` (recommended: 60 s). After expiry, push stops silently.
- Change detection is implementation-defined. The hub may fall back to polling if a node pushes too frequently.

---

## 10. Radio Interface (implementation contract)

Any radio backend must provide these three operations to the protocol layer. All operations are **non-blocking** from the protocol layer's perspective:

```
Init() error
    Configure the radio with the BleRiot RF parameters (§2),
    set the hardware receive address to this device's own address,
    and enter receive mode.

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
