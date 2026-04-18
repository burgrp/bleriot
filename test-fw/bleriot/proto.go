// Package bleriot implements the BleRiot IoT register protocol.
// All operations are allocation-free; packets are passed by value or via
// caller-supplied buffers.
package bleriot

import (
	"encoding/binary"
)

// PacketSize is the fixed size of every BleRiot packet on the wire.
const PacketSize = 12

// FLAGS byte bit positions.
const (
	FlagOP   uint8 = 1 << 0 // 0=read, 1=write
	FlagDIR  uint8 = 1 << 1 // 0=request, 1=response
	FlagPUSH uint8 = 1 << 2 // 0=one-shot, 1=subscribe
)

// Defined FLAGS combinations.
const (
	FlagsReadReq     uint8 = 0x00 // hub → node: read register (one-shot)
	FlagsReadReqPush uint8 = 0x04 // hub → node: read register + subscribe
	FlagsWriteReq    uint8 = 0x01 // hub → node: write register
	FlagsReadResp    uint8 = 0x02 // node → hub: read response / push notification
	FlagsWriteResp   uint8 = 0x03 // node → hub: write response (echoes actual value)
	FlagsPush        uint8 = 0x06 // node → hub: unsolicited push (SEQ must be SeqPush)
)

// SeqPush is the reserved sequence number used by unsolicited PUSH packets.
const SeqPush uint8 = 0xFF

// Address is a BleRiot device address (CRC32 of the hardware UID).
// The zero value (0x00000000) is reserved and must not be assigned to any device.
type Address uint32

// Bytes returns the address as 4 bytes in little-endian order, as transmitted
// on the wire and written to radio hardware registers.
func (a Address) Bytes() [4]byte {
	return [4]byte{byte(a), byte(a >> 8), byte(a >> 16), byte(a >> 24)}
}

// Packet is a decoded BleRiot packet. All fields are plain values; no pointers.
// The destination is not in the payload — it is carried as the RF sync word
// (hardware filtering) and is always equal to the receiver's own address.
type Packet struct {
	SRC   Address
	FLAGS uint8
	SEQ   uint8
	REG   uint16
	VALUE int32
}

// Encode serialises p into the 12-byte wire format.
// buf must be exactly PacketSize bytes.
func (p *Packet) Encode(buf *[PacketSize]byte) {
	binary.LittleEndian.PutUint32(buf[0:4], uint32(p.SRC))
	buf[4] = p.FLAGS
	buf[5] = p.SEQ
	binary.LittleEndian.PutUint16(buf[6:8], p.REG)
	binary.LittleEndian.PutUint32(buf[8:12], uint32(p.VALUE))
}

// Decode deserialises 12 wire bytes into p.
// buf must be exactly PacketSize bytes.
func (p *Packet) Decode(buf *[PacketSize]byte) {
	p.SRC = Address(binary.LittleEndian.Uint32(buf[0:4]))
	p.FLAGS = buf[4]
	p.SEQ = buf[5]
	p.REG = binary.LittleEndian.Uint16(buf[6:8])
	p.VALUE = int32(binary.LittleEndian.Uint32(buf[8:12]))
}

// IsRequest returns true for hub→node packets.
func (p *Packet) IsRequest() bool { return p.FLAGS&FlagDIR == 0 }

// IsResponse returns true for node→hub packets.
func (p *Packet) IsResponse() bool { return p.FLAGS&FlagDIR != 0 }

// IsRead returns true for read operations (request or response).
func (p *Packet) IsRead() bool { return p.FLAGS&FlagOP == 0 }

// IsWrite returns true for write operations (request or response).
func (p *Packet) IsWrite() bool { return p.FLAGS&FlagOP != 0 }

// IsPush returns true if the PUSH subscription flag is set.
func (p *Packet) IsPush() bool { return p.FLAGS&FlagPUSH != 0 }

// Radio is the interface that any BleRiot-compatible radio backend must implement.
// See PROTOCOL.md §10 for the contract.
type Radio interface {
	// Init configures the radio with BleRiot RF parameters and enters RX mode.
	Init() error

	// Send transmits one packet to dst. dst is the destination device address
	// (4 bytes, little-endian) used as the RF sync word so the receiver's
	// hardware filter accepts the packet. Blocks only for the air transmission.
	// Does not wait for a response.
	Send(dst [4]byte, payload []byte) error

	// Receive checks whether a packet is waiting. If yes, copies up to
	// len(buf) bytes and returns (n, true). If no packet, returns (0, false)
	// immediately without blocking.
	Receive(buf []byte) (int, bool)
}

// Stack is a stateless BleRiot protocol handler. It encodes outgoing packets and
// decodes incoming ones. No state, no timers, no allocations.
// Reliability (timeouts, retries) and subscription tracking are the caller's
// responsibility.
type Stack struct {
	radio      Radio
	ownAddress Address
	buf        [PacketSize]byte // reused for every encode/decode; safe because single-goroutine
}

// NewStack creates a Stack bound to radio and ownAddress.
func NewStack(radio Radio, ownAddress Address) Stack {
	return Stack{radio: radio, ownAddress: ownAddress}
}

// SendReadRequest transmits a READ_REQ to dst for register reg.
// subscribe=true sets the PUSH flag, asking the node to send future changes.
func (s *Stack) SendReadRequest(dst Address, seq uint8, reg uint16, subscribe bool) error {
	p := Packet{
		SRC:   s.ownAddress,
		FLAGS: FlagsReadReq,
		SEQ:   seq,
		REG:   reg,
	}
	if subscribe {
		p.FLAGS = FlagsReadReqPush
	}
	p.Encode(&s.buf)
	return s.radio.Send(dst.Bytes(), s.buf[:])
}

// SendWriteRequest transmits a WRITE_REQ to dst for register reg with value.
func (s *Stack) SendWriteRequest(dst Address, seq uint8, reg uint16, value int32) error {
	p := Packet{
		SRC:   s.ownAddress,
		FLAGS: FlagsWriteReq,
		SEQ:   seq,
		REG:   reg,
		VALUE: value,
	}
	p.Encode(&s.buf)
	return s.radio.Send(dst.Bytes(), s.buf[:])
}

// SendReadResponse transmits a READ_RESP from this node to dst.
func (s *Stack) SendReadResponse(dst Address, seq uint8, reg uint16, value int32) error {
	p := Packet{
		SRC:   s.ownAddress,
		FLAGS: FlagsReadResp,
		SEQ:   seq,
		REG:   reg,
		VALUE: value,
	}
	p.Encode(&s.buf)
	return s.radio.Send(dst.Bytes(), s.buf[:])
}

// SendWriteResponse transmits a WRITE_RESP from this node to dst with the actual value.
func (s *Stack) SendWriteResponse(dst Address, seq uint8, reg uint16, actualValue int32) error {
	p := Packet{
		SRC:   s.ownAddress,
		FLAGS: FlagsWriteResp,
		SEQ:   seq,
		REG:   reg,
		VALUE: actualValue,
	}
	p.Encode(&s.buf)
	return s.radio.Send(dst.Bytes(), s.buf[:])
}

// SendPush transmits an unsolicited PUSH notification to dst.
func (s *Stack) SendPush(dst Address, reg uint16, value int32) error {
	p := Packet{
		SRC:   s.ownAddress,
		FLAGS: FlagsPush,
		SEQ:   SeqPush,
		REG:   reg,
		VALUE: value,
	}
	p.Encode(&s.buf)
	return s.radio.Send(dst.Bytes(), s.buf[:])
}

// Receive attempts to read one packet from the radio. Returns (Packet, true)
// if a complete BleRiot packet is available, (zero, false) otherwise. Non-blocking.
// Hardware filtering guarantees that any received packet is addressed to this device.
func (s *Stack) Receive() (Packet, bool) {
	n, ok := s.radio.Receive(s.buf[:])
	if !ok || n != PacketSize {
		return Packet{}, false
	}
	var p Packet
	p.Decode(&s.buf)
	return p, true
}
