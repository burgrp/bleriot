package node

import (
	"errors"
	"testing"

	"github.com/burgrp/bleriot/lib/shared/protocol"
)

type fakeRadio struct {
	rx      [][]byte
	sent    []sentPacket
	events  *[]string
	sendErr error
}

type sentPacket struct {
	dst    [4]byte
	packet []byte
}

func (r *fakeRadio) Send(dst [4]byte, packet []byte) error {
	if r.events != nil {
		*r.events = append(*r.events, "send")
	}
	copyPacket := make([]byte, len(packet))
	copy(copyPacket, packet)
	r.sent = append(r.sent, sentPacket{dst: dst, packet: copyPacket})
	return r.sendErr
}

func (r *fakeRadio) Receive(buf []byte) (int, bool) {
	if len(r.rx) == 0 {
		return 0, false
	}
	packet := r.rx[0]
	r.rx = r.rx[1:]
	return copy(buf, packet), true
}

type fakeDevice struct {
	value      int32
	null       bool
	writes     int
	writtenTag uint16
	written    int32
	wroteNull  bool
	events     *[]string
}

func (d *fakeDevice) Read(tag uint16) (int32, bool) {
	if tag != 1 {
		return 0, true
	}
	return d.value, d.null
}

func (d *fakeDevice) Write(tag uint16, value int32, null bool) {
	if d.events != nil {
		*d.events = append(*d.events, "write")
	}
	d.writes++
	d.writtenTag = tag
	d.written = value
	d.wroteNull = null
	if tag == 1 {
		d.value = value
		d.null = null
	}
}

var (
	testKey  = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	nodeSelf = [4]byte{0xDE, 0xAD, 0xBE, 0xEF}
	hubAddr  = [4]byte{0xFF, 0xFF, 0xFF, 0x01}
)

func encodeRequest(t *testing.T, typ, flags byte, reg uint16, value int32) []byte {
	t.Helper()
	codec, err := protocol.NewCodec(testKey)
	if err != nil {
		t.Fatalf("NewCodec: %v", err)
	}
	packet := make([]byte, protocol.PacketLen)
	codec.Encode(packet, hubAddr, typ, flags, reg, value)
	return packet
}

func decodeResponse(t *testing.T, packet []byte) (src [4]byte, typ, flags byte, reg uint16, value int32) {
	t.Helper()
	codec, err := protocol.NewCodec(testKey)
	if err != nil {
		t.Fatalf("NewCodec: %v", err)
	}
	src, typ, flags, reg, value, err = codec.Decode(packet)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return
}

func newTestNode(t *testing.T, dev Device) (*Node, *fakeRadio) {
	t.Helper()
	radio := &fakeRadio{}
	n, err := New(radio, nodeSelf, testKey, dev)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return n, radio
}

func TestGetRepliesWithCurrentValue(t *testing.T) {
	dev := &fakeDevice{value: 42}
	n, radio := newTestNode(t, dev)
	requestFlags := protocol.FlagsWithGuard(protocol.FlagNULL, 1)
	radio.rx = append(radio.rx, encodeRequest(t, protocol.TypeGET, requestFlags, 1, 0))

	if !n.Poll() {
		t.Fatal("Poll did not consume the request")
	}
	if len(radio.sent) != 1 {
		t.Fatalf("sent %d packets, want 1", len(radio.sent))
	}

	src, typ, flags, reg, value := decodeResponse(t, radio.sent[0].packet)
	if src != nodeSelf || radio.sent[0].dst != hubAddr {
		t.Fatalf("response route = %X -> %X, want %X -> %X", src, radio.sent[0].dst, nodeSelf, hubAddr)
	}
	if typ != protocol.TypeVALUE || reg != 1 || value != 42 {
		t.Fatalf("response type/reg/value = %#x/%d/%d, want VALUE/1/42", typ, reg, value)
	}
	if flags != protocol.GuardFlags(requestFlags) {
		t.Fatalf("response flags = %#x, want echoed GUARD %#x with request NULL cleared", flags, protocol.GuardFlags(requestFlags))
	}
}

func TestGetNullReturnsNullAndZeroValue(t *testing.T) {
	n, radio := newTestNode(t, &fakeDevice{value: 99, null: true})
	requestFlags := protocol.FlagsWithGuard(0, 1)
	radio.rx = append(radio.rx, encodeRequest(t, protocol.TypeGET, requestFlags, 1, 0))

	n.Poll()
	_, typ, flags, reg, value := decodeResponse(t, radio.sent[0].packet)
	if typ != protocol.TypeVALUE || reg != 1 || value != 0 {
		t.Fatalf("response type/reg/value = %#x/%d/%d, want VALUE/1/0", typ, reg, value)
	}
	if flags != protocol.GuardFlags(requestFlags)|protocol.FlagNULL {
		t.Fatalf("response flags = %#x, want echoed GUARD|NULL", flags)
	}
}

func TestUnknownTagReturnsNull(t *testing.T) {
	n, radio := newTestNode(t, &fakeDevice{})
	radio.rx = append(radio.rx, encodeRequest(t, protocol.TypeGET, 0, 7, 0))

	n.Poll()
	_, typ, flags, reg, value := decodeResponse(t, radio.sent[0].packet)
	if typ != protocol.TypeVALUE || flags != protocol.FlagNULL || reg != 7 || value != 0 {
		t.Fatalf("response = type %#x flags %#x reg %d value %d, want VALUE/NULL/7/0", typ, flags, reg, value)
	}
}

func TestSetAcksBeforeWriteAndEchoesFlags(t *testing.T) {
	events := make([]string, 0, 2)
	dev := &fakeDevice{events: &events}
	radio := &fakeRadio{events: &events}
	n, err := New(radio, nodeSelf, testKey, dev)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	requestFlags := protocol.FlagsWithGuard(protocol.FlagNULL, 1)
	radio.rx = append(radio.rx, encodeRequest(t, protocol.TypeSET, requestFlags, 1, 123))

	n.Poll()
	if len(events) != 2 || events[0] != "send" || events[1] != "write" {
		t.Fatalf("events = %v, want [send write]", events)
	}
	if dev.writes != 1 || dev.writtenTag != 1 || dev.written != 123 || !dev.wroteNull {
		t.Fatalf("write = count %d tag %d value %d null %v", dev.writes, dev.writtenTag, dev.written, dev.wroteNull)
	}
	src, typ, flags, reg, value := decodeResponse(t, radio.sent[0].packet)
	if src != nodeSelf || radio.sent[0].dst != hubAddr {
		t.Fatalf("ACK route = %X -> %X, want %X -> %X", src, radio.sent[0].dst, nodeSelf, hubAddr)
	}
	if typ != protocol.TypeACK || flags != requestFlags || reg != 1 || value != 0 {
		t.Fatalf("ACK = type %#x flags %#x reg %d value %d, want ACK/%#x/1/0", typ, flags, reg, value, requestFlags)
	}
}

func TestDuplicateSetRepeatsIdempotentAssignment(t *testing.T) {
	dev := &fakeDevice{}
	n, radio := newTestNode(t, dev)
	request := encodeRequest(t, protocol.TypeSET, 0, 1, 77)
	radio.rx = append(radio.rx, request, request)

	n.Poll()
	n.Poll()
	if dev.writes != 2 || dev.value != 77 {
		t.Fatalf("writes/value = %d/%d, want 2/77", dev.writes, dev.value)
	}
	if len(radio.sent) != 2 {
		t.Fatalf("sent %d ACKs, want 2", len(radio.sent))
	}
}

func TestSetWritesAfterAckSendFailure(t *testing.T) {
	events := make([]string, 0, 2)
	dev := &fakeDevice{events: &events}
	radio := &fakeRadio{events: &events, sendErr: errors.New("radio unavailable")}
	n, err := New(radio, nodeSelf, testKey, dev)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	radio.rx = append(radio.rx, encodeRequest(t, protocol.TypeSET, 0, 1, 456))

	if !n.Poll() {
		t.Fatal("Poll did not consume SET")
	}
	if len(events) != 2 || events[0] != "send" || events[1] != "write" {
		t.Fatalf("events = %v, want [send write]", events)
	}
	if dev.writes != 1 || dev.written != 456 {
		t.Fatalf("write count/value = %d/%d, want 1/456", dev.writes, dev.written)
	}
}

func TestNodeToHubPacketTypesAreSilent(t *testing.T) {
	dev := &fakeDevice{}
	n, radio := newTestNode(t, dev)
	radio.rx = append(radio.rx,
		encodeRequest(t, protocol.TypeVALUE, 0, 1, 10),
		encodeRequest(t, protocol.TypeACK, 0, 1, 0),
	)

	if !n.Poll() || !n.Poll() {
		t.Fatal("Poll did not consume node-to-hub packets")
	}
	if dev.writes != 0 || len(radio.sent) != 0 {
		t.Fatalf("writes/responses = %d/%d, want 0/0", dev.writes, len(radio.sent))
	}
}

func TestUnknownPacketTypeIsSilent(t *testing.T) {
	dev := &fakeDevice{}
	n, radio := newTestNode(t, dev)
	radio.rx = append(radio.rx, encodeRequest(t, 0xFE, 0, 1, 10))

	if !n.Poll() {
		t.Fatal("Poll did not consume unknown packet type")
	}
	if dev.writes != 0 || len(radio.sent) != 0 {
		t.Fatalf("writes/responses = %d/%d, want 0/0", dev.writes, len(radio.sent))
	}
}

func TestUnsupportedPacketVersionIsSilent(t *testing.T) {
	n, radio := newTestNode(t, &fakeDevice{})
	request := encodeRequest(t, protocol.TypeGET, 0, 1, 0)
	request[4] = protocol.PacketVersion + 1
	radio.rx = append(radio.rx, request)

	if n.Poll() {
		t.Fatal("unsupported packet version reported as handled")
	}
	if len(radio.sent) != 0 {
		t.Fatalf("unsupported packet version produced %d responses, want 0", len(radio.sent))
	}
}

func TestPollIgnoresMissingAndMalformedPackets(t *testing.T) {
	n, radio := newTestNode(t, &fakeDevice{})
	if n.Poll() {
		t.Fatal("empty radio reported a handled packet")
	}
	radio.rx = append(radio.rx, make([]byte, protocol.PacketLen-1))
	if n.Poll() {
		t.Fatal("short packet reported as handled")
	}
	if len(radio.sent) != 0 {
		t.Fatalf("malformed input produced %d responses, want 0", len(radio.sent))
	}
}
