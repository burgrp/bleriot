# BleRiot Protocol Specification

Hardware-independent specification for the BleRiot IoT register protocol.

> This is also the README of the [`lib`](.) module
> (`github.com/burgrp/bleriot/lib`), the single library that implements this
> specification. The module is split into:
>
> - [`shared`](shared) — neutral, dependency-free, build-tag-free packages used by
>   both firmware and host: `protocol` (packet codec + XTEA, §4–§8), `config`
>   (identity primitives and constants, §11.5), `inventory` (the
>   inventory-as-code model, §11), and `puya` (PY32 chip profiles and memory maps).
> - [`node`](node) — the firmware-side runtime (receive/dispatch loop, §7–§8).
> - [`site`](site) — the host hub library; see [site/README.md](site/README.md).

---

## 1. Overview

BleRiot is a hub-initiated request/response protocol for reading and writing
named integer registers on low-power RF nodes.

- **Topology:** star — one hub, many nodes
- **Initiator:** hub only
- **Register type:** `int32` (signed 32-bit integer)
- **Transactions:** `GET` → `VALUE` and idempotent absolute `SET` → `ACK`

Nodes are divided into independent groups by RF channel. Each group has one
half-duplex RX/TX radio and at most one in-flight transaction. Groups on
different channels transact concurrently.

---

## 2. RF Physical Layer

These parameters define the current BleRiot radio link:

| Parameter | Value |
|---|---|
| Channel | One channel number and spread factor per independent node group |
| Sync word | Four-byte destination address (§3) |
| Data rate | 250 kbps |
| Modulation | GFSK |
| Framing | PAN211x BLE-compatible raw packet format: preamble, sync word, PDU, three-byte CRC, and whitening |
| PDU size | 13 bytes, fixed |

This is not a standard BLE connection. BleRiot selects raw RF channels and uses
its own packet, addressing, transaction, and encryption rules.

Each node's channel and spread factor are baked into its firmware (§11.5). All
nodes in a channel group use the same pair. The hub assigns one half-duplex radio
to each active group and may operate multiple groups concurrently.

For every transmission, the radio sync word is the destination's four-byte
address. A node configures its hardware receive address to its own address; hub
radios configure theirs to the hub address. The destination is therefore
filtered by the radio and is not repeated in the PDU.

---

## 3. Device Addressing

Each node has a random **four-byte address**, generated for its inventory entry
and baked into its firmware image. Addresses are opaque byte arrays and must be
unique within a deployment. The hub also has a four-byte source/receive address,
configured at runtime.

Reserved address: `0x00000000` — must not be assigned to any device.

---

## 4. Packet Format

All packets share the same fixed 13-byte structure:

```text
Offset  Size  Field
──────  ────  ─────────────────────────────────────────────────
0       4     SRC   — source device address (little-endian, plaintext)
4       1     VER   — packet format version 0x01 (plaintext)
5       8     BLOCK — XTEA encrypted block (see §5):
                        TYPE  (1 byte)  — packet type (see §6)
                        FLAGS (1 byte)  — NULL and GUARD (see §6)
                        REG   (2 bytes) — register address (uint16, little-endian)
                        VALUE (4 bytes) — int32, little-endian
──────  ────
Total: 13 bytes
```

The destination is not carried in the payload. It is encoded as the RF sync word (§2), which the receiver's hardware uses for filtering. A received packet is therefore always addressed to the receiving device.

`SRC` and `VER` are plaintext. The hub uses `SRC` to select the provisioned node
and its shared key before decrypting `BLOCK`; unknown source addresses are
discarded. Version values other than `0x01` are rejected.

All multi-byte fields inside BLOCK are **little-endian**.

---

## 5. Security

The eight-byte `BLOCK` is encrypted with 32-round **XTEA** using the node's
16-byte shared key (§11.5), interpreted as four little-endian `uint32` words.
The hub and that node use the same key in both directions.

There is no nonce, transaction token, message authentication code, or replay
protection. Plaintext `SRC` and `VER` are routing and format fields, not
authenticated metadata.

---

## 6. TYPE and FLAGS

### 6.1 TYPE

| Value | Name | Sender | Meaning |
|---:|---|---|---|
| `0x00` | `GET` | hub | Read the current register value. |
| `0x01` | `VALUE` | node | Return a register value in direct response to `GET`. |
| `0x02` | `SET` | hub | Apply an idempotent absolute register assignment. |
| `0x03` | `ACK` | node | Confirm receipt of a `SET` request. |

The node consumes packets with other type values without responding. The host
accepts only `VALUE` and `ACK` as response types.

### 6.2 NULL

`FLAGS` bit 0 is `NULL`; its meaning follows packet direction and type:

| Packet | `NULL=0` | `NULL=1` |
|---|---|---|
| `GET` | Normal request. | Not used by the host; the node's reply is still determined only by `Device.Read`. |
| `VALUE` | `VALUE` carries the current raw `int32`. | The register has no value; the receiver ignores `VALUE`, and the node encodes it as zero. |
| `SET` | `VALUE` is the absolute raw assignment. | Clear assignment; `VALUE` is undefined and the host encodes it as zero. |
| `ACK` | Ordinary `SET` received. | Clear `SET` received; echoes the request's `NULL` bit. |

### 6.3 GUARD

`FLAGS` bits 7–1 encode `GUARD`, an unsigned delay from 0 to 127 ms. The host
sets it on every `GET` and `SET` from the assigned radio's transmit-to-receive
turnaround requirement. After receiving a valid request, the node waits this
guard before **every** `VALUE` or `ACK`, allowing the half-duplex hub radio to
return to receive mode.

Both response types echo the request's GUARD bits as pacing metadata. Echoed
bits never request another wait. `VALUE` takes `NULL` only from `Device.Read`;
`ACK` echoes the `SET` request's `NULL` bit.

---

## 7. Transactions

### 7.1 Get

```text
Hub  -> Node    TYPE=GET    REG=R  VALUE=0
Node -> Hub     TYPE=VALUE  REG=R  VALUE=<current raw int32>
```

The node calls `Device.Read(R)` and returns its value and absence state. An
unknown register is a device policy decision; the runtime passes the tag to the
device unchanged.

### 7.2 Set

```text
Hub  -> Node    TYPE=SET  REG=R  VALUE=<absolute raw int32>
Node -> Hub     TYPE=ACK  REG=R  VALUE=0
```

After receiving `SET`, the node waits GUARD and sends `ACK` before calling
`Device.Write(R, value, null)`. The ACK confirms packet receipt only. `Write`
accepts or starts the assignment; the physical action may continue
asynchronously after it returns. Unknown and read-only tags may be ignored by
the device implementation, but their syntactically valid SET packets are still
acknowledged by the protocol runtime.

`SET` is an idempotent absolute assignment because a lost response causes the
hub to transmit the same request again, which may call `Device.Write` again.
Device writes must therefore make duplicate calls harmless. `ACK` confirms
neither acceptance nor completion of the requested action. The bridge publishes
only values observed by later `GET` transactions; while an action is in
progress, those GETs may return the previous or an intermediate state.

The node emits exactly one direct response per handled request and sends no
unsolicited protocol packets.

---

## 8. Channel Ownership and Response Matching

One channel group has one persistent transaction lane, even when its physical
radio is disconnected or replaced. A transaction owns that lane while queued
work is excluded, from its initial send through all retries, reply waits, and
any required response drain. Transactions on other channels use independent
lanes and proceed concurrently.

There is no transaction token in the packet. While a transaction owns its
channel, the host accepts a response only when all of these match:

- plaintext `SRC` selects the expected provisioned node and key;
- decoded `REG` equals the requested register;
- decoded type is `VALUE` for `GET` or `ACK` for `SET`.

Other valid responses are classified as orphans and cannot complete the active
or a later transaction. Unknown response types, unsupported versions, and
decode failures are rejected.

---

## 9. Reliability and Turnaround

RF delivery is best effort; the host provides bounded request retransmission:

1. Send the request and wait one per-attempt timeout for a matching response.
2. If the wait expires, retransmit the identical request and wait again.
3. After all attempts expire, retain channel ownership for a bounded response
  drain, consuming late responses, then return a timeout.

The default per-attempt timeout is 50 ms. The default retry count is three
retransmissions after the initial send, for at most four sends. A send error
is not retried. A first-attempt success releases the channel immediately. After
a retry succeeds, the engine drains responses before release because more than
one request may have produced a valid reply. Cancellation before any successful
send returns immediately; cancellation after a successful send also drains
before release. Caller cancellation cannot abort a required drain.

The assigned radio reports its reply guard. The host encodes its whole-millisecond
value, clamped to 127 ms, on every request. Registering a radio fails when its
guard plus 10 ms of minimum reply headroom exceeds the configured timeout.
The drain interval is the larger of 10 ms and the radio guard plus that 10 ms
headroom, so it covers both host receive latency and the latest valid node
turnaround while remaining bounded by the configured timeout.

Because retries repeat an absolute `SET`, acknowledgment loss cannot turn a
write into a relative operation. A response arriving during the drain cannot
satisfy the next transaction; response matching still requires source,
register, and type.

---

## 10. Host Polling, Liveness, and Registry Publication

The host bridge runs one scheduler worker per RF channel. Each round polls every
register of every node assigned to that channel, serially through the channel's
transaction lane. The configurable sweep interval is a target period for a
complete channel round and defaults to one second. If a round takes longer, the
next starts immediately; rounds never overlap.

Each round rotates which node starts first. Every node in the round's snapshot
completes before any node is repeated, so an overloaded channel does not always
favor the same first node. Registers within a node are polled in descriptor
order. Registry-originated `SET` calls use the same channel lane.

Each Registry provider starts with `nil`. A successful `GET` is converted and
published; protocol `NULL` bypasses conversion and publishes `nil`. A decode
error also publishes `nil`, but the response still counts as node activity.
Failed GETs retain the register's last publication until node-level liveness
declares the node unavailable.

Liveness uses consecutive wholly unsuccessful node sweeps. The default
threshold is three. Any successful `GET`, or an acknowledged `SET`, resets the
node's failure count. On reaching the threshold, the bridge publishes `nil` for
all registers once. Later successes clear the offline state, and successful
GETs republish current values.

Writable Registry requests are encoded and sent as ordinary or `NULL` `SET`
transactions. Read-only requests, including `NULL`, are ignored. An `ACK` is not
published optimistically; a subsequent scheduled GET supplies the Registry value.

---

## 11. Register Model and Node Identity

The protocol carries registers as bare `uint16` IDs on the wire (§4). Names,
types, access modes, conversions, and metadata are **hub-side knowledge** and are deliberately
absent from the wire format for performance and memory reasons. The mapping
between wire IDs and named, typed registers lives in the host as
**inventory-as-code**: a Go program that the host library compiles and runs.
There is no runtime descriptor exchange or generated register descriptor. The
only generated artifact is the firmware entry point that bakes in one inventory
instance's identity and config (§11.5).

This section defines the register model (tags, types, access modes, conversions), the
inventory-as-code authoring model (device types and instances), and the
node-identity path (baked into the firmware image).

### 11.1 Concepts

| Concept | Role | Source of truth | Carries wire IDs? |
|---------|------|-----------------|-------------------|
| **Device type** | A reusable, named register table (`Name` + `Registers`). Defines register *names*, tags, types, access modes, conversions, and metadata. | Hand-authored Go (device-type module's `Type()`) | Yes (tags) |
| **Instance** | One physical device: its name, address, key, channel, device type, and config. Describes *what a node is and where it is*. | Hand-authored Go (the site inventory) | n/a |
| **Node identity** | The node's random `address` (§3) and shared `key` (§5). Per-device; the key is secret. | Generated by `bleriot new`, stored in inventory, and baked into firmware (§11.5) | n/a |

The single most important rule: **a register's wire identity is its hand-assigned
`Tag`** (a `uint16`, like a protobuf field number) — unique and non-zero within a
device type, and **never reused** once retired. Because tags are stable by
construction, firmware and host cannot drift: reordering or extending a register
table never changes any existing register's wire ID.

```
device type (Go)                    ──┐
 (tag, name, access, conversion)      │
                                      ├─► host runtime ──► Registry bridge
instance (Go)                         │
 (address, key, channel, config)     ─┘
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

| Field      | Type                       | Description                                                   |
|------------|----------------------------|---------------------------------------------------------------|
| tag        | uint16                     | Permanent wire identity: unique, non-zero, never reused       |
| name       | string                     | Register name, unique within the device type                  |
| type       | enum                       | Registry-facing `int`, `float`, or `bool`                     |
| readOnly   | bool                       | Ignore all Registry writes when true                          |
| conversion | `inventory.Conversion`     | Optional raw-to-Registry and Registry-to-raw functions        |
| metadata   | map<string,string>         | Key-value pairs merged into the hub's register record         |

All registers carry `int32` on the wire (§8). `type`, `readOnly`, and
`conversion` are hub-side only: the node always sends and receives raw `int32`.
They are ordinary Go values in the device type's `Type()` function; they are not
serialized into `main_gen.go` or sent over the radio.

`Conversion` contains two functions:

| Function | Direction | Contract |
|----------|-----------|----------|
| `Decode func(int32) (any, error)` | node raw value → Registry value | Called for successful non-NULL GETs |
| `Encode func(any) (int32, error)` | Registry request → node raw value | Called before a non-NULL SET |

The zero `Conversion` selects the natural conversion for `type`: `int32` maps
to `int64`, nonzero maps to `bool`, and `int32` maps to `float64`. A writable
custom conversion must provide both `Decode` and `Encode`. A read-only register
may provide `Decode` alone and must not provide `Encode`; it may also use the
zero conversion for default decoding. Inventory validation rejects every other
combination. The hub ignores all consumer writes to a read-only register,
including NULL writes, and publishes `readOnly: true` in its Registry metadata.
The firmware's `Device.Write` should still ignore that tag so the node enforces
the same hardware policy independently.

A NULL node value bypasses `Decode` and is published as `nil`. If `Decode`
returns an error for a non-NULL raw value, the bridge logs the raw value and
also publishes `nil`. The node did answer, so this is not treated as transport
loss and does not start repeated seeding GETs.

The [`shared/conversion`](shared/conversion) package supplies invertible linear
factories. `Scale(factor)` exposes `raw*factor`; `Linear(factor, offset)` exposes
`raw*factor+offset`. Encoding performs the inverse, rounds to the nearest raw
integer, and saturates at the `int32` limits. For example, a writable
temperature stored in hundredths of a degree can expose degrees directly:

```go
inventory.Register{
  Tag:        4,
  Name:       "setpoint",
  Type:       inventory.TypeFloat,
  Conversion: conversion.Scale(0.01),
}
```

Nonlinear one-way conversions can keep sensor mathematics off the node. The
[`shared/conversion/ntc`](shared/conversion/ntc) package implements the NTC beta
equation and exposes degrees Celsius from a raw ADC code:

```go
inventory.Register{
  Tag:      5,
  Name:     "temperature",
  Type:     inventory.TypeFloat,
  ReadOnly: true,
  Conversion: ntc.Beta(ntc.BetaParams{
    ADCMax:              4095,
    FixedResistance:     10_000,
    NominalResistance:   10_000,
    NominalTemperatureC: 25,
    Beta:                3950,
    Position:            ntc.ThermistorLowSide,
  }),
  Metadata: map[string]string{"unit": "celsius"},
}
```

`ThermistorLowSide` means `Vref -- fixed -- ADC -- NTC -- ground`;
`ThermistorHighSide` means `Vref -- NTC -- ADC -- fixed -- ground`. The factory
assumes the divider supply is the ADC reference, so the measurement is
ratiometric. Fixed parameters are checked when `Beta` is called. Raw endpoint
codes (`0` and `ADCMax`) return a decode error because they imply zero or
infinite thermistor resistance.

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
| address  | [4]byte   | Random, nonzero RF address; unique within the inventory (§3)     |
| key      | [16]byte  | XTEA shared key (§5)                                             |
| channel  | Channel   | RF channel (number + spread factor) the device uses (§2); declared once and shared across instances |
| type     | DeviceType| The device's register table (§11.2)                             |
| config   | any       | Device-type-specific configuration baked into the firmware image |

A `Channel` bundles a required, unique `Name`, the RF `Number`, and the
`SpreadFactor` every node on it uses (a dongle transmits one factor at a time, so
binding the two prevents two nodes on one channel from disagreeing). The zero
`SpreadFactor` is the highest-range S8.

The host validates the inventory at startup: tags are unique and non-zero within
each device type, instance names and nonzero addresses are unique, and each
channel uses a single spreading factor.

### 11.5 Node Identity

`address` (§3) and `key` (§5) are **not** part of any device type. They are
per-device and the key is secret, so they are **baked into the firmware image** for
one specific device, together with the device's RF channel, spread factor and
config. The host generates a small Go file that supplies these values to the
firmware's `bleriotMain` entry point — `bleriot make` writes it as part of
building, and `bleriot gen` emits the same source to stdout; there is no
provisioning page in flash. `new` generates the address and key locally without
contacting a device; `gen` looks the device up in the inventory (by name, or the
sole instance) and bakes those stored values in alongside the channel and config.

The identity travels as a `node.Provisioning` value (address, key, channel,
spread factor) plus the device type's `Config`. The generated file is a trivial
`main()` that calls `bleriotMain(prov, cfg)`; everything else is hand-written
firmware.

### 11.6 Worked Example

A `bob` device type, authored in Go (tags hand-assigned, permanent):

```go
inventory.DeviceType{
    Name: "bob",
    Registers: []inventory.Register{
    {Tag: 1, Name: "green", Type: inventory.TypeInt},
    {Tag: 2, Name: "red", Type: inventory.TypeInt},
    {Tag: 3, Name: "gpio", Type: inventory.TypeInt},
    },
}
```

A site inventory instantiates it per physical device:

```go
inventory.Inventory{
    {
        Name:    "bob",
      Address: [4]byte{ /* random address generated by new */ },
        Key:     [16]byte{ /* XTEA key */ },
        Channel: inventory.Channel{Name: "far", Number: 37},
        Type:    bob.Type(),
        Config:  bob.Config{DefaultRedPeriod: 500, DefaultGreenPeriod: 100},
    },
}
```

The host uses `bob`'s stored address, maps each register's tag to its wire REG,
and publishes `bob.green`, `bob.red` and `bob.gpio` to the
Registry. Two instances of the same type coexist because their *names* differ;
the wire never sees the instance concept.

### 11.7 Onboarding and Build Workflow

1. **Onboard.** Run `new`: the host generates a random nonzero address and XTEA
  key and prints a paste-ready `Instance{}` stub without contacting hardware.
  Fill in the name, channel, type and config, and commit it to inventory.
2. **Build + flash.** Run `make <name> flash` (name the instance, or rely on the
   sole one): the host writes a generated Go file that bakes the device's identity
  (address, key, channel, spread factor) and config into the
   firmware source, injects the chip's build/flash targets, and runs the device
  module's Makefile to build and flash the image over SWD. (`gen` emits the same
  generated source to stdout for inspection.)
3. **Run.** Run `hub`: the host builds every inventory device's register
  descriptor and bridges its registers to the Registry.

Adding or updating a device is an edit to the inventory **source**, type-checked
by the Go compiler — there are no JSON files to hand-edit and no descriptor pool
to keep in sync.

---

## 12. Radio Interface (implementation contract)

Firmware uses a packet radio with these operations:

```text
Send(dst [4]byte, payload []byte) error
Receive(buf []byte) (n int, ok bool)
```

`Send` sets the destination sync word and transmits one complete packet.
`Receive` is non-blocking and copies at most one available packet. The firmware
configures channel, spread factor, and its receive address before constructing
the node runtime.

The host engine uses `Send`, an asynchronous stream of complete received
13-byte packets, and `ReplyGuard() time.Duration`. A host adapter may poll a
physical dongle internally, but it presents complete packets to the engine. The
protocol layer assumes no underlying host transport beyond this contract.
