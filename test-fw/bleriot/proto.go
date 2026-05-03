// Package bleriot implements the BleRiot IoT register protocol.
// All operations are allocation-free; packets are passed by value or via
// caller-supplied buffers.
package bleriot

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

// PacketLen is the fixed on-wire packet size in bytes.
const PacketLen = 20

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

// Codec encrypts and decrypts BleRiot packets using AES-128-ECB with the
// node's shared key. Create one per node key via NewCodec; reuse for all packets.
type Codec struct {
	block cipher.Block
}

// NewCodec creates a Codec for the given 16-byte AES-128 key.
// Allocates the AES key schedule once; subsequent Encode/Decode calls are
// allocation-free.
func NewCodec(key [16]byte) (Codec, error) {
	b, err := aes.NewCipher(key[:])
	if err != nil {
		return Codec{}, err
	}
	return Codec{block: b}, nil
}

// Encode writes a 20-byte encrypted packet into dst[0:PacketLen].
//
// Packet layout:
//
//	[0:4]  SRC   — plaintext source address
//	[4:20] BLOCK — AES-128-ECB encrypted: TYPE(1)+FLAGS(1)+REG(2)+VALUE(4)+NONCE(8)
//
// nonce must be 8 bytes of caller-supplied random data, unique per packet.
func (c *Codec) Encode(dst []byte, src [4]byte, typ, flags byte, reg uint16, value int32, nonce [8]byte) {
	copy(dst[0:4], src[:])

	var plain [16]byte
	plain[0] = typ
	plain[1] = flags
	plain[2] = byte(reg)
	plain[3] = byte(reg >> 8)
	plain[4] = byte(value)
	plain[5] = byte(value >> 8)
	plain[6] = byte(value >> 16)
	plain[7] = byte(value >> 24)
	copy(plain[8:16], nonce[:])

	c.block.Encrypt(dst[4:20], plain[:])
}

// Decode decrypts and parses a 20-byte packet. Returns errShortPacket if
// len(raw) < PacketLen.
func (c *Codec) Decode(raw []byte) (src [4]byte, typ, flags byte, reg uint16, value int32, err error) {
	if len(raw) < PacketLen {
		err = errShortPacket
		return
	}

	copy(src[:], raw[0:4])

	var plain [16]byte
	c.block.Decrypt(plain[:], raw[4:20])

	typ = plain[0]
	flags = plain[1]
	reg = uint16(plain[2]) | uint16(plain[3])<<8
	v := uint32(plain[4]) | uint32(plain[5])<<8 | uint32(plain[6])<<16 | uint32(plain[7])<<24
	value = int32(v)
	return
}
