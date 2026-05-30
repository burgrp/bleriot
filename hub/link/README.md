# hub/link — host↔modem link protocol

A standalone, dependency-free Go module defining the **COBS-framed serial
protocol** spoken between the BleRiot [host hub](../../cli/README.md) and the MCU
[radio modem](../fw/README.md) (UART now, USB-CDC later).

The module imports only `errors` and uses no build tags, so the exact same
source compiles unchanged into both the Linux host and the TinyGo firmware,
single-sourcing the wire framing.

> Module path: `hub/link`.

---

## Framing

Frames are delimited by a single **zero byte** (`Delimiter = 0x00`). Each frame
body is **COBS-encoded** so it never contains a zero, giving unambiguous frame
boundaries and cheap resynchronisation after line noise.

```
┌────────────────── COBS-encoded body ──────────────────┐
│ [code] [data ...] (no zero bytes)                      │ 0x00
└────────────────────────────────────────────────────────┘  ▲ delimiter
```

- `AppendMessage(dst, msg)` marshals a message body and appends a full frame
  (encoded body + delimiter).
- `Decoder` (`NewDecoder(maxFrame)`) is fed one byte at a time via `Push(b)`;
  it returns a decoded `Message` when a complete frame arrives, resyncing on the
  next delimiter after a malformed frame.

---

## Messages

A message is one type byte plus a type-specific body.

| Type | Value | Direction | Body | Purpose |
|------|-------|-----------|------|---------|
| `MsgHello` | `0x01` | modem → host | `[Version]` | Announce on boot; lets the host detect a firmware/protocol mismatch. |
| `MsgConfigRadio` | `0x02` | host → modem | `[Channel][Addr0..3]` | Set the radio's channel and receive address. |
| `MsgSend` | `0x03` | host → modem | `[Dst0..3][Payload...]` | Transmit one BleRiot packet to a destination address. |
| `MsgRecv` | `0x04` | modem → host | `[Payload...]` | Deliver one received BleRiot packet. |
| `MsgError` | `0x05` | modem → host | `[Code]` | Report a modem-side error. |

`ProtocolVersion = 0x01` is the current link version carried by `MsgHello`.
`AddrLen = 4` is the BleRiot device address length.

### Error codes (`MsgError`)

| Code | Name | Meaning |
|------|------|---------|
| `0x00` | `ErrNone` | No error. |
| `0x01` | `ErrTxFailed` | Radio reported a transmit failure. |
| `0x02` | `ErrBadFrame` | Malformed or short command frame. |
| `0x03` | `ErrUnknownType` | Unrecognised `MsgType`. |

---

## Design notes

- **One modem == one radio == one serial port.** No radio index appears on the
  wire; the host fans out across several modems and does the multiplexing above
  this layer.
- `Message.Payload`, when present, **aliases the decoder's internal buffer** and
  is only valid until the next decode. Copy it if you need to retain it.
- The encoder is allocation-friendly; the firmware reuses a single TX buffer and
  never allocates in its run loop.

This is the transport for the [BleRiot protocol](../../protocol/README.md); the
payloads it carries are opaque 13-byte BleRiot packets.
