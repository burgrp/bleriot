// Package protocol implements the BleRiot IoT register protocol.
// All operations are allocation-free; packets are passed by value or via
// caller-supplied buffers.
package protocol

import "errors"

// PacketLen is the fixed on-wire packet size in bytes.
const PacketLen = 13

// PacketVersion is the current plaintext packet format version.
const PacketVersion byte = 0x00

// Packet TYPE values (§7).
const (
	TypeGET   byte = 0x00 // hub → node: read register (one-shot)
	TypeSET   byte = 0x01 // hub → node: write register
	TypeIS    byte = 0x02 // node → hub: current register value
	TypeWATCH byte = 0x03 // hub → node: subscribe (VALUE=1) or unsubscribe (VALUE=0)
	TypeACK   byte = 0x04 // acknowledges a SET (node→hub) or a push (hub→node); no value
)

// FLAGS bits (§6).
const FlagNULL byte = 0x01 // VALUE is absent; register has no value

// FlagPush marks a node→hub IS as a spontaneous push (from Notify) that the hub
// must acknowledge with an ACK, distinguishing it from a solicited IS reply to a
// GET or WATCH. It occupies FLAGS bit 1. GUARD (below) also spans bits 1–7, but
// only on hub→node requests; on node→hub IS packets the GUARD field is unused,
// so bit 1 is free to carry PUSH in that direction. The hub recovers a lost
// solicited reply by retransmitting its request, but a spontaneous push has no
// outstanding request behind it, so it carries its own acknowledgement instead.
const FlagPush byte = 0x02

// Reply turnaround guard (§6, §9). FLAGS bits 1–7 carry GUARD: the number of
// milliseconds a node waits, after receiving a request, before it transmits its
// reply. It gives a slow half-duplex hub radio time to switch from transmit back
// to receive so it does not miss the answer. The hub sets GUARD on every request
// (GET/SET/WATCH); replies (node → hub) leave it zero. 0 means reply immediately.
const (
	guardShift          = 1
	guardMask      byte = 0x7F // 7 bits → 0–127 ms, in FLAGS bits 1–7
	MaxGuardMillis      = guardMask
)

// FlagsWithGuard packs guardMillis (clamped to MaxGuardMillis) into the GUARD
// field of flags, preserving the low NULL bit.
func FlagsWithGuard(flags, guardMillis byte) byte {
	if guardMillis > MaxGuardMillis {
		guardMillis = MaxGuardMillis
	}
	return (flags &^ (guardMask << guardShift)) | (guardMillis << guardShift)
}

// GuardMillis returns the reply turnaround guard (milliseconds) encoded in the
// GUARD field of a FLAGS byte.
func GuardMillis(flags byte) byte {
	return (flags >> guardShift) & guardMask
}

var (
	errShortPacket       = errors.New("short packet")
	errUnsupportedPacket = errors.New("unsupported packet version")
)

// Codec encrypts and decrypts BleRiot packets using XTEA with the node's
// shared key. Create one per node key via NewCodec; reuse for all packets.
// Zero allocation after construction.
type Codec struct {
	key [4]uint32
}

// NewCodec creates a Codec for the given 16-byte key (interpreted as 4×uint32 LE).
func NewCodec(key [16]byte) (Codec, error) {
	var c Codec
	for i := range c.key {
		c.key[i] = uint32(key[i*4]) | uint32(key[i*4+1])<<8 |
			uint32(key[i*4+2])<<16 | uint32(key[i*4+3])<<24
	}
	return c, nil
}

// Encode writes a 13-byte packet into dst[0:PacketLen].
//
// Packet layout:
//
//	[0:4]  SRC   — plaintext source address
//	[4]    VER   — packet format version
//	[5:13] BLOCK — XTEA encrypted: TYPE(1)+FLAGS(1)+REG(2)+VALUE(4)
func (c *Codec) Encode(dst []byte, src [4]byte, typ, flags byte, reg uint16, value int32) {
	copy(dst[0:4], src[:])
	dst[4] = PacketVersion

	v0 := uint32(typ) | uint32(flags)<<8 | uint32(reg)<<16
	v1 := uint32(value)
	xteaEncrypt(&v0, &v1, &c.key)
	dst[5] = byte(v0)
	dst[6] = byte(v0 >> 8)
	dst[7] = byte(v0 >> 16)
	dst[8] = byte(v0 >> 24)
	dst[9] = byte(v1)
	dst[10] = byte(v1 >> 8)
	dst[11] = byte(v1 >> 16)
	dst[12] = byte(v1 >> 24)
}

// Decode decrypts and parses a 13-byte packet. Returns errShortPacket if
// len(raw) < PacketLen.
func (c *Codec) Decode(raw []byte) (src [4]byte, typ, flags byte, reg uint16, value int32, err error) {
	if len(raw) < PacketLen {
		err = errShortPacket
		return
	}

	if raw[4] != PacketVersion {
		err = errUnsupportedPacket
		return
	}

	copy(src[:], raw[0:4])

	v0 := uint32(raw[5]) | uint32(raw[6])<<8 | uint32(raw[7])<<16 | uint32(raw[8])<<24
	v1 := uint32(raw[9]) | uint32(raw[10])<<8 | uint32(raw[11])<<16 | uint32(raw[12])<<24
	xteaDecrypt(&v0, &v1, &c.key)
	typ = byte(v0)
	flags = byte(v0 >> 8)
	reg = uint16(v0 >> 16)
	value = int32(v1)
	return
}

const xteaDelta uint32 = 0x9E3779B9

func xteaEncrypt(v0, v1 *uint32, key *[4]uint32) {
	var sum uint32
	for range 32 {
		*v0 += ((*v1<<4 ^ *v1>>5) + *v1) ^ (sum + key[sum&3])
		sum += xteaDelta
		*v1 += ((*v0<<4 ^ *v0>>5) + *v0) ^ (sum + key[sum>>11&3])
	}
}

func xteaDecrypt(v0, v1 *uint32, key *[4]uint32) {
	var sum uint32 = 0xC6EF3720 // xteaDelta * 32, pre-computed to avoid compile-time overflow
	for range 32 {
		*v1 -= ((*v0<<4 ^ *v0>>5) + *v0) ^ (sum + key[sum>>11&3])
		sum -= xteaDelta
		*v0 -= ((*v1<<4 ^ *v1>>5) + *v1) ^ (sum + key[sum&3])
	}
}
