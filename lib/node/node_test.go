package node

import (
	"errors"
	"testing"
	"time"

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

func TestPollWithStatusHeartbeatBoundaries(t *testing.T) {
	n, _ := newTestNode(t, &fakeDevice{})
	tests := []struct {
		name        string
		now         time.Duration
		wantLED     bool
		wantChanged bool
	}{
		{name: "initial", now: 0, wantLED: true, wantChanged: true},
		{name: "end of on interval", now: 199 * time.Millisecond, wantLED: true},
		{name: "start of off interval", now: 200 * time.Millisecond, wantChanged: true},
		{name: "end of off interval", now: 999 * time.Millisecond},
		{name: "next cycle", now: time.Second, wantLED: true, wantChanged: true},
		{name: "same phase", now: 1100 * time.Millisecond, wantLED: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			online, led, changed := n.statusAt(test.now)
			if online || led != test.wantLED || changed != test.wantChanged {
				t.Fatalf("status = online %v led %v changed %v, want false/%v/%v", online, led, changed, test.wantLED, test.wantChanged)
			}
		})
	}
}

func TestPollWithStatusHeartbeatIsIndependentOfOnline(t *testing.T) {
	n, _ := newTestNode(t, &fakeDevice{})
	n.receivedAt(0)

	online, led, changed := n.statusAt(0)
	if !online || !led || !changed {
		t.Fatalf("initial status = %v/%v/%v, want true/true/true", online, led, changed)
	}

	online, led, changed = n.statusAt(statusLEDOn)
	if !online || led || !changed {
		t.Fatalf("status at LED off edge = %v/%v/%v, want true/false/true", online, led, changed)
	}
}

func TestPollWithStatusOnlineTimeoutAndRefresh(t *testing.T) {
	n, _ := newTestNode(t, &fakeDevice{})
	n.receivedAt(0)

	online, led, changed := n.statusAt(0)
	if !online || !led || !changed {
		t.Fatalf("initial received status = %v/%v/%v, want true/true/true", online, led, changed)
	}

	n.receivedAt(4 * time.Second)
	online, led, changed = n.statusAt(4 * time.Second)
	if !online || !led || changed {
		t.Fatalf("refreshed status = %v/%v/%v, want true/true/false", online, led, changed)
	}

	online, led, changed = n.statusAt(8 * time.Second)
	if !online || !led || changed {
		t.Fatalf("status before refreshed deadline = %v/%v/%v, want true/true/false", online, led, changed)
	}

	online, led, changed = n.statusAt(9 * time.Second)
	if online || !led || !changed {
		t.Fatalf("status at refreshed deadline = %v/%v/%v, want false/true/true", online, led, changed)
	}

	n.receivedAt(10 * time.Second)
	online, led, changed = n.statusAt(10 * time.Second)
	if !online || !led || !changed {
		t.Fatalf("recovered status = %v/%v/%v, want true/true/true", online, led, changed)
	}
}

func TestPollWithStatusUsesPollResultForLiveness(t *testing.T) {
	n, radio := newTestNode(t, &fakeDevice{})

	online, led, changed := n.PollWithStatus()
	if online || !led || !changed {
		t.Fatalf("initial status = %v/%v/%v, want false/true/true", online, led, changed)
	}

	radio.rx = append(radio.rx, encodeRequest(t, protocol.TypeGET, 0, 1, 0))
	online, _, changed = n.PollWithStatus()
	if !online || !changed {
		t.Fatalf("status after valid request = online %v changed %v, want true/true", online, changed)
	}

	n, radio = newTestNode(t, &fakeDevice{})
	radio.rx = append(radio.rx, encodeRequest(t, protocol.TypeACK, 0, 1, 0))
	online, _, changed = n.PollWithStatus()
	if !online || !changed {
		t.Fatalf("status after silently consumed packet = online %v changed %v, want true/true", online, changed)
	}
	if len(radio.sent) != 0 {
		t.Fatalf("silently consumed packet produced %d responses, want 0", len(radio.sent))
	}

	n, radio = newTestNode(t, &fakeDevice{})
	radio.rx = append(radio.rx, encodeRequest(t, protocol.TypeGET, 0, 1, 0))
	if !n.Poll() {
		t.Fatal("valid request was not handled")
	}
	deadline := n.onlineUntil
	online, _, changed = n.PollWithStatus()
	if !online || !changed {
		t.Fatalf("status after Poll = online %v changed %v, want true/true", online, changed)
	}

	unsupportedVersion := encodeRequest(t, protocol.TypeGET, 0, 1, 0)
	unsupportedVersion[4] = protocol.PacketVersion + 1
	invalidPackets := []struct {
		name   string
		packet []byte
	}{
		{name: "short", packet: make([]byte, protocol.PacketLen-1)},
		{name: "unsupported version", packet: unsupportedVersion},
	}
	for _, invalid := range invalidPackets {
		t.Run(invalid.name, func(t *testing.T) {
			radio.rx = append(radio.rx, invalid.packet)
			if n.Poll() {
				t.Fatal("invalid packet reported as handled")
			}
			if n.onlineUntil != deadline {
				t.Fatalf("invalid packet moved deadline from %v to %v", deadline, n.onlineUntil)
			}
		})
	}
	online, _, changed = n.statusAt(deadline)
	if online || !changed {
		t.Fatalf("malformed packet extended liveness: online %v changed %v", online, changed)
	}
}

func TestPollWithStatusDoesNotAllocate(t *testing.T) {
	n, _ := newTestNode(t, &fakeDevice{})
	n.PollWithStatus()

	if allocs := testing.AllocsPerRun(1000, func() {
		n.PollWithStatus()
	}); allocs != 0 {
		t.Fatalf("PollWithStatus allocated %v times per call, want 0", allocs)
	}
}
