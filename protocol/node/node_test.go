package node

import (
	"testing"

	"protocol"
)

// fakeRadio records sent packets and feeds queued packets to Receive. It lets a
// test drive the runtime deterministically: enqueue a request, Poll, inspect the
// reply.
type fakeRadio struct {
	rx   [][]byte     // packets waiting to be Received, FIFO
	sent []sentPacket // packets the Node transmitted
}

type sentPacket struct {
	dst    [4]byte
	packet []byte
}

func (r *fakeRadio) Send(dst [4]byte, packet []byte) error {
	cp := make([]byte, len(packet))
	copy(cp, packet)
	r.sent = append(r.sent, sentPacket{dst: dst, packet: cp})
	return nil
}

func (r *fakeRadio) Receive(buf []byte) (int, bool) {
	if len(r.rx) == 0 {
		return 0, false
	}
	pkt := r.rx[0]
	r.rx = r.rx[1:]
	n := copy(buf, pkt)
	return n, true
}

// fakeDevice is a trivial two-register device: tag 1 is a writable int, tag 2 is
// read-only and reports null. Any other tag is unknown (null).
type fakeDevice struct {
	value     int32
	written   int32
	wroteNull bool
}

func (d *fakeDevice) Read(tag uint16) (int32, bool) {
	switch tag {
	case 1:
		return d.value, false
	default:
		return 0, true
	}
}

func (d *fakeDevice) Write(tag uint16, value int32, null bool) {
	switch tag {
	case 1:
		d.wroteNull = null
		if null {
			return
		}
		d.written = value
		d.value = value
	}
}

var (
	testKey  = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	nodeSelf = [4]byte{0xDE, 0xAD, 0xBE, 0xEF}
	hubAddr  = [4]byte{0xFF, 0xFF, 0xFF, 0x01}
)

// encodeReq builds a hub→node request packet with the test key.
func encodeReq(t *testing.T, typ byte, reg uint16, value int32) []byte {
	return encodeReqFlags(t, typ, 0, reg, value)
}

// encodeReqFlags is encodeReq with an explicit FLAGS byte (e.g. FlagNULL).
func encodeReqFlags(t *testing.T, typ, flags byte, reg uint16, value int32) []byte {
	t.Helper()
	c, err := protocol.NewCodec(testKey)
	if err != nil {
		t.Fatalf("NewCodec: %v", err)
	}
	buf := make([]byte, protocol.PacketLen)
	c.Encode(buf, hubAddr, typ, flags, reg, value)
	return buf
}

// decodeReply decrypts a packet the Node sent.
func decodeReply(t *testing.T, raw []byte) (src [4]byte, typ, flags byte, reg uint16, value int32) {
	t.Helper()
	c, err := protocol.NewCodec(testKey)
	if err != nil {
		t.Fatalf("NewCodec: %v", err)
	}
	src, typ, flags, reg, value, err = c.Decode(raw)
	if err != nil {
		t.Fatalf("Decode reply: %v", err)
	}
	return
}

func newTestNode(t *testing.T, dev Device) (*Node, *fakeRadio) {
	t.Helper()
	r := &fakeRadio{}
	n, err := New(r, nodeSelf, testKey, dev)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return n, r
}

func TestGetRepliesWithValue(t *testing.T) {
	dev := &fakeDevice{value: 42}
	n, r := newTestNode(t, dev)

	r.rx = append(r.rx, encodeReq(t, protocol.TypeGET, 1, 0))
	if !n.Poll() {
		t.Fatal("Poll did not handle the GET")
	}
	if len(r.sent) != 1 {
		t.Fatalf("sent %d packets, want 1", len(r.sent))
	}

	src, typ, flags, reg, value := decodeReply(t, r.sent[0].packet)
	if typ != protocol.TypeIS {
		t.Fatalf("reply type = %#x, want IS", typ)
	}
	if src != nodeSelf {
		t.Fatalf("reply SRC = %X, want node self", src)
	}
	if r.sent[0].dst != hubAddr {
		t.Fatalf("reply dst = %X, want hub", r.sent[0].dst)
	}
	if flags&protocol.FlagNULL != 0 {
		t.Fatal("reply unexpectedly NULL")
	}
	if reg != 1 || value != 42 {
		t.Fatalf("reply reg/value = %d/%d, want 1/42", reg, value)
	}
}

func TestSetAppliesAndAcks(t *testing.T) {
	dev := &fakeDevice{}
	n, r := newTestNode(t, dev)

	r.rx = append(r.rx, encodeReq(t, protocol.TypeSET, 1, 99))
	n.Poll()

	if dev.written != 99 {
		t.Fatalf("device written = %d, want 99", dev.written)
	}
	src, typ, _, reg, _ := decodeReply(t, r.sent[0].packet)
	if typ != protocol.TypeACK {
		t.Fatalf("reply type = %#x, want ACK", typ)
	}
	if src != nodeSelf {
		t.Fatalf("ack SRC = %X, want node self", src)
	}
	if r.sent[0].dst != hubAddr {
		t.Fatalf("ack dst = %X, want hub", r.sent[0].dst)
	}
	if reg != 1 {
		t.Fatalf("ack reg = %d, want 1", reg)
	}
}

func TestSetNullPassesNullToDevice(t *testing.T) {
	dev := &fakeDevice{value: 7}
	n, r := newTestNode(t, dev)

	r.rx = append(r.rx, encodeReqFlags(t, protocol.TypeSET, protocol.FlagNULL, 1, 0))
	n.Poll()

	if !dev.wroteNull {
		t.Fatal("device.Write did not receive null=true")
	}
	// The NULL write was ignored by the device, so the value is unchanged.
	if dev.value != 7 {
		t.Fatalf("value = %d, want unchanged 7", dev.value)
	}
	// A SET is still acknowledged with an ACK.
	_, typ, _, _, _ := decodeReply(t, r.sent[0].packet)
	if typ != protocol.TypeACK {
		t.Fatalf("reply type = %#x, want ACK", typ)
	}
}

func TestUnknownTagRepliesNull(t *testing.T) {
	n, r := newTestNode(t, &fakeDevice{})

	r.rx = append(r.rx, encodeReq(t, protocol.TypeGET, 7, 0))
	n.Poll()

	_, typ, flags, reg, value := decodeReply(t, r.sent[0].packet)
	if typ != protocol.TypeIS {
		t.Fatalf("reply type = %#x, want IS", typ)
	}
	if flags&protocol.FlagNULL == 0 {
		t.Fatal("reply should have NULL flag for unknown tag")
	}
	if reg != 7 || value != 0 {
		t.Fatalf("reply reg/value = %d/%d, want 7/0", reg, value)
	}
}

func TestWatchSubscribesAndPushes(t *testing.T) {
	dev := &fakeDevice{value: 5}
	n, r := newTestNode(t, dev)

	// Subscribe (WATCH value=1): expect an immediate IS with the current value.
	r.rx = append(r.rx, encodeReq(t, protocol.TypeWATCH, 1, 1))
	n.Poll()
	if len(r.sent) != 1 {
		t.Fatalf("subscribe sent %d packets, want 1 immediate IS", len(r.sent))
	}
	_, typ, _, reg, value := decodeReply(t, r.sent[0].packet)
	if typ != protocol.TypeIS || reg != 1 || value != 5 {
		t.Fatalf("immediate IS = type %#x reg %d value %d, want IS/1/5", typ, reg, value)
	}

	// A change pushes a new IS to the subscriber.
	dev.value = 8
	n.Notify(1, 8, false)
	if len(r.sent) != 2 {
		t.Fatalf("Notify sent total %d packets, want 2", len(r.sent))
	}
	dst := r.sent[1].dst
	if dst != hubAddr {
		t.Fatalf("push dst = %X, want hub", dst)
	}
	_, typ, _, reg, value = decodeReply(t, r.sent[1].packet)
	if typ != protocol.TypeIS || reg != 1 || value != 8 {
		t.Fatalf("push IS = type %#x reg %d value %d, want IS/1/8", typ, reg, value)
	}
}

func TestNotifyWithoutSubscriberIsSilent(t *testing.T) {
	n, r := newTestNode(t, &fakeDevice{})
	n.Notify(1, 123, false)
	if len(r.sent) != 0 {
		t.Fatalf("Notify with no subscribers sent %d packets, want 0", len(r.sent))
	}
}

func TestNotifyNullPushesNullFlag(t *testing.T) {
	dev := &fakeDevice{value: 5}
	n, r := newTestNode(t, dev)

	r.rx = append(r.rx, encodeReq(t, protocol.TypeWATCH, 1, 1)) // subscribe
	n.Poll()
	before := len(r.sent)

	n.Notify(1, 99, true) // value is ignored when null
	if len(r.sent) != before+1 {
		t.Fatalf("Notify(null) sent %d packets, want 1", len(r.sent)-before)
	}
	_, typ, flags, reg, value := decodeReply(t, r.sent[before].packet)
	if typ != protocol.TypeIS || reg != 1 {
		t.Fatalf("push = type %#x reg %d, want IS/1", typ, reg)
	}
	if flags&protocol.FlagNULL == 0 {
		t.Fatalf("push flags = %#x, want NULL set", flags)
	}
	if value != 0 {
		t.Fatalf("push value = %d, want 0 when null", value)
	}
}

func TestUnsubscribeStopsPushes(t *testing.T) {
	dev := &fakeDevice{value: 1}
	n, r := newTestNode(t, dev)

	r.rx = append(r.rx, encodeReq(t, protocol.TypeWATCH, 1, 1)) // subscribe
	n.Poll()
	r.rx = append(r.rx, encodeReq(t, protocol.TypeWATCH, 1, 0)) // unsubscribe
	n.Poll()

	before := len(r.sent)
	n.Notify(1, 77, false)
	if len(r.sent) != before {
		t.Fatalf("Notify after unsubscribe sent %d extra packets, want 0", len(r.sent)-before)
	}
}

func TestNotifyOnlyTargetsMatchingTag(t *testing.T) {
	dev := &fakeDevice{value: 1}
	n, r := newTestNode(t, dev)

	r.rx = append(r.rx, encodeReq(t, protocol.TypeWATCH, 1, 1))
	n.Poll()
	sentAfterSub := len(r.sent)

	n.Notify(2, 50, false) // different tag, no subscriber
	if len(r.sent) != sentAfterSub {
		t.Fatalf("Notify on unwatched tag pushed %d packets, want 0", len(r.sent)-sentAfterSub)
	}
}
