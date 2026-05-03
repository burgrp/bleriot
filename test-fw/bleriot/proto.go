// Package bleriot implements the BleRiot IoT register protocol.
// All operations are allocation-free; packets are passed by value or via
// caller-supplied buffers.
package bleriot

import "errors"

// PacketLen is the fixed on-wire packet size in bytes.
const PacketLen = 12

// Packet TYPE values (§7).
const (
	TypeGET   byte = 0x00 // hub → node: read register (one-shot)
	TypeSET   byte = 0x01 // hub → node: write register
	TypeIS    byte = 0x02 // node → hub: current register value
	TypeWATCH byte = 0x03 // hub → node: subscribe (VALUE=1) or unsubscribe (VALUE=0)
)

// FLAGS bits (§6).
const FlagNULL byte = 0x01 // VALUE is absent; register has no value

var errShortPacket = errors.New("short packet")

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

// Encode writes a 12-byte encrypted packet into dst[0:PacketLen].
//
// Packet layout:
//
//	[0:4]  SRC   — plaintext source address
//	[4:12] BLOCK — XTEA encrypted: TYPE(1)+FLAGS(1)+REG(2)+VALUE(4)
func (c *Codec) Encode(dst []byte, src [4]byte, typ, flags byte, reg uint16, value int32) {
	copy(dst[0:4], src[:])

	v0 := uint32(typ) | uint32(flags)<<8 | uint32(reg)<<16
	v1 := uint32(value)
	xteaEncrypt(&v0, &v1, &c.key)
	dst[4] = byte(v0); dst[5] = byte(v0 >> 8); dst[6] = byte(v0 >> 16); dst[7] = byte(v0 >> 24)
	dst[8] = byte(v1); dst[9] = byte(v1 >> 8); dst[10] = byte(v1 >> 16); dst[11] = byte(v1 >> 24)
}

// Decode decrypts and parses a 20-byte packet. Returns errShortPacket if
// len(raw) < PacketLen.
func (c *Codec) Decode(raw []byte) (src [4]byte, typ, flags byte, reg uint16, value int32, err error) {
	if len(raw) < PacketLen {
		err = errShortPacket
		return
	}

	copy(src[:], raw[0:4])

	v0 := uint32(raw[4]) | uint32(raw[5])<<8 | uint32(raw[6])<<16 | uint32(raw[7])<<24
	v1 := uint32(raw[8]) | uint32(raw[9])<<8 | uint32(raw[10])<<16 | uint32(raw[11])<<24
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
