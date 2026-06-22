package engine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"protocol"
	"site/node"
)

var testKey = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

const (
	testChannel uint8 = 10
	regTemp           = uint16(0x1234)
)

var (
	hubAddr  = [4]byte{0xCC, 0xA0, 0x00, 0x01}
	nodeAddr = [4]byte{0xCC, 0xA0, 0x00, 0x02}
)

// fakeRadio captures sent packets and lets a simulated node inject replies.
type fakeRadio struct {
	sent  chan [PacketLen]byte
	recv  chan [PacketLen]byte
	drop  int           // drop the first n Send calls (to exercise retries)
	guard time.Duration // reply turnaround guard reported to the engine
	mu    sync.Mutex
}

func newFakeRadio() *fakeRadio {
	return &fakeRadio{
		sent: make(chan [PacketLen]byte, 16),
		recv: make(chan [PacketLen]byte, 16),
	}
}

func (f *fakeRadio) Send(dst [4]byte, payload []byte) error {
	f.mu.Lock()
	drop := f.drop > 0
	if drop {
		f.drop--
	}
	f.mu.Unlock()
	var p [PacketLen]byte
	copy(p[:], payload)
	if !drop {
		f.sent <- p
	}
	return nil
}

func (f *fakeRadio) Received() <-chan [PacketLen]byte { return f.recv }

func (f *fakeRadio) ReplyGuard() time.Duration { return f.guard }

// simulateNode reads one request, decodes it, and replies: an ACK for a SET
// (§8.2), or an IS for a GET/WATCH. reply transforms the request value into the
// response value (used only for the IS reply).
func simulateNode(t *testing.T, f *fakeRadio, c protocol.Codec, reply func(typ byte, reg uint16, val int32) (int32, bool)) {
	t.Helper()
	go func() {
		for req := range f.sent {
			_, typ, _, reg, val, err := c.Decode(req[:])
			if err != nil {
				t.Errorf("node decode: %v", err)
				continue
			}
			rv, null := reply(typ, reg, val)
			var resp [PacketLen]byte
			if typ == protocol.TypeSET {
				c.Encode(resp[:], nodeAddr, protocol.TypeACK, 0, reg, 0)
				f.recv <- resp
				continue
			}
			flags := byte(0)
			if null {
				flags = protocol.FlagNULL
			}
			c.Encode(resp[:], nodeAddr, protocol.TypeIS, flags, reg, rv)
			f.recv <- resp
		}
	}()
}

func newEngine(t *testing.T) (*Engine, *fakeRadio, protocol.Codec, context.CancelFunc) {
	t.Helper()
	c, err := protocol.NewCodec(testKey)
	if err != nil {
		t.Fatal(err)
	}
	e := New(Options{HubAddr: hubAddr, Timeout: 50 * time.Millisecond, Retries: 3})
	f := newFakeRadio()
	ctx, cancel := context.WithCancel(context.Background())
	if err := e.AddRadio(ctx, testChannel, f); err != nil {
		t.Fatal(err)
	}

	n := node.NewNode(
		"t",
		testChannel,
		&node.Descriptor{},
		node.Identity{Address: nodeAddr, Key: testKey},
	)
	if err := e.AddNode(n); err != nil {
		t.Fatal(err)
	}
	return e, f, c, cancel
}

func TestEngine_Get(t *testing.T) {
	e, f, c, cancel := newEngine(t)
	defer cancel()
	simulateNode(t, f, c, func(typ byte, reg uint16, val int32) (int32, bool) {
		return 4242, false
	})

	u, err := e.Get(context.Background(), nodeAddr, regTemp)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if u.Value != 4242 || u.Null {
		t.Fatalf("Get = %+v, want {4242 false}", u)
	}
}

// TestEngine_RequestCarriesGuard checks the engine packs the radio's reply
// turnaround guard (protocol/README.md §6) into the GUARD field of every request.
func TestEngine_RequestCarriesGuard(t *testing.T) {
	c, err := protocol.NewCodec(testKey)
	if err != nil {
		t.Fatal(err)
	}
	e := New(Options{HubAddr: hubAddr, Timeout: 50 * time.Millisecond, Retries: 3})
	f := newFakeRadio()
	f.guard = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := e.AddRadio(ctx, testChannel, f); err != nil {
		t.Fatal(err)
	}
	n := node.NewNode("t", testChannel, &node.Descriptor{}, node.Identity{Address: nodeAddr, Key: testKey})
	if err := e.AddNode(n); err != nil {
		t.Fatal(err)
	}

	// Reply so the transaction completes; assert the request's GUARD field.
	go func() {
		for req := range f.sent {
			_, _, flags, reg, _, derr := c.Decode(req[:])
			if derr != nil {
				t.Errorf("decode: %v", derr)
				continue
			}
			if got := protocol.GuardMillis(flags); got != 20 {
				t.Errorf("request GUARD = %d ms, want 20", got)
			}
			var resp [PacketLen]byte
			c.Encode(resp[:], nodeAddr, protocol.TypeIS, 0, reg, 1)
			f.recv <- resp
		}
	}()

	if _, err := e.Get(context.Background(), nodeAddr, regTemp); err != nil {
		t.Fatalf("Get: %v", err)
	}
}

// TestEngine_AddRadioRejectsLargeGuard checks AddRadio refuses a radio whose
// reply guard leaves no headroom under the timeout (protocol/README.md §6:
// GUARD < T_timeout), so the misconfiguration surfaces at startup instead of as
// silent, total packet loss.
func TestEngine_AddRadioRejectsLargeGuard(t *testing.T) {
	e := New(Options{HubAddr: hubAddr, Timeout: 20 * time.Millisecond, Retries: 3})
	f := newFakeRadio()
	f.guard = 20 * time.Millisecond // == timeout: no room for the reply to arrive
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := e.AddRadio(ctx, testChannel, f); !errors.Is(err, ErrGuardTooLarge) {
		t.Fatalf("AddRadio error = %v, want ErrGuardTooLarge", err)
	}
}

func TestEngine_Set(t *testing.T) {
	e, f, c, cancel := newEngine(t)
	defer cancel()
	simulateNode(t, f, c, func(typ byte, reg uint16, val int32) (int32, bool) {
		if typ != protocol.TypeSET {
			t.Errorf("expected SET, got %d", typ)
		}
		return val, false
	})

	if err := e.Set(context.Background(), nodeAddr, regTemp, 250); err != nil {
		t.Fatalf("Set: %v", err)
	}
}

func TestEngine_SetNull(t *testing.T) {
	e, f, c, cancel := newEngine(t)
	defer cancel()

	// Simulate a node that checks the request carries the NULL flag, then ACKs.
	go func() {
		for req := range f.sent {
			_, typ, flags, reg, _, err := c.Decode(req[:])
			if err != nil {
				t.Errorf("node decode: %v", err)
				continue
			}
			if typ != protocol.TypeSET {
				t.Errorf("expected SET, got %d", typ)
			}
			if flags&protocol.FlagNULL == 0 {
				t.Errorf("expected NULL flag, got flags %#x", flags)
			}
			var resp [PacketLen]byte
			c.Encode(resp[:], nodeAddr, protocol.TypeACK, 0, reg, 0)
			f.recv <- resp
		}
	}()

	if err := e.SetNull(context.Background(), nodeAddr, regTemp); err != nil {
		t.Fatalf("SetNull: %v", err)
	}
}

func TestEngine_NullResponse(t *testing.T) {
	e, f, c, cancel := newEngine(t)
	defer cancel()
	simulateNode(t, f, c, func(typ byte, reg uint16, val int32) (int32, bool) {
		return 0, true
	})

	u, err := e.Get(context.Background(), nodeAddr, regTemp)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !u.Null {
		t.Fatal("expected Null update")
	}
}

func TestEngine_RetriesThenSucceeds(t *testing.T) {
	e, f, c, cancel := newEngine(t)
	defer cancel()
	f.mu.Lock()
	f.drop = 2 // drop first two attempts; third should succeed
	f.mu.Unlock()
	simulateNode(t, f, c, func(typ byte, reg uint16, val int32) (int32, bool) {
		return 7, false
	})

	u, err := e.Get(context.Background(), nodeAddr, regTemp)
	if err != nil {
		t.Fatalf("Get after retries: %v", err)
	}
	if u.Value != 7 {
		t.Fatalf("value = %d, want 7", u.Value)
	}
}

func TestEngine_Timeout(t *testing.T) {
	e, _, _, cancel := newEngine(t)
	defer cancel()
	// No node simulator: never replies.
	_, err := e.Get(context.Background(), nodeAddr, regTemp)
	if err != ErrTimeout {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
}

func TestEngine_UnknownNode(t *testing.T) {
	e, _, _, cancel := newEngine(t)
	defer cancel()
	_, err := e.Get(context.Background(), [4]byte{9, 9, 9, 9}, regTemp)
	if err != ErrUnknownNode {
		t.Fatalf("err = %v, want ErrUnknownNode", err)
	}
}

func TestEngine_Watch(t *testing.T) {
	e, f, c, cancel := newEngine(t)
	defer cancel()
	simulateNode(t, f, c, func(typ byte, reg uint16, val int32) (int32, bool) {
		return 11, false // immediate value on WATCH
	})

	var mu sync.Mutex
	var got []int32
	err := e.Watch(context.Background(), nodeAddr, regTemp, func(u Update) {
		mu.Lock()
		got = append(got, u.Value)
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	// Simulate an unsolicited push.
	var push [PacketLen]byte
	c.Encode(push[:], nodeAddr, protocol.TypeIS, 0, regTemp, 22)
	f.recv <- push

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected >=2 callback values, got %v", got)
		case <-time.After(5 * time.Millisecond):
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if got[0] != 11 || got[1] != 22 {
		t.Fatalf("callback values = %v, want [11 22]", got)
	}
}

func TestEngine_RefreshReWatches(t *testing.T) {
	c, err := protocol.NewCodec(testKey)
	if err != nil {
		t.Fatal(err)
	}
	// Short refresh interval so the test runs quickly.
	e := New(Options{HubAddr: hubAddr, Timeout: 50 * time.Millisecond, Retries: 3, RefreshInterval: 20 * time.Millisecond})
	f := newFakeRadio()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := e.AddRadio(ctx, testChannel, f); err != nil {
		t.Fatal(err)
	}
	n := node.NewNode(
		"t",
		testChannel,
		&node.Descriptor{},
		node.Identity{Address: nodeAddr, Key: testKey},
	)
	if err := e.AddNode(n); err != nil {
		t.Fatal(err)
	}

	// Count WATCH requests the node sees.
	var mu sync.Mutex
	var watches int
	simulateNode(t, f, c, func(typ byte, reg uint16, val int32) (int32, bool) {
		if typ == protocol.TypeWATCH && val == 1 {
			mu.Lock()
			watches++
			mu.Unlock()
		}
		return 1, false
	})

	if err := e.Watch(context.Background(), nodeAddr, regTemp, func(Update) {}); err != nil {
		t.Fatalf("Watch: %v", err)
	}

	go e.Run(ctx)

	// Expect the initial WATCH plus several refreshes.
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		w := watches
		mu.Unlock()
		if w >= 4 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected >=4 WATCH requests from refresh, got %d", w)
		case <-time.After(5 * time.Millisecond):
		}
	}

	// After Unwatch, refreshes must stop targeting the register.
	if err := e.Unwatch(context.Background(), nodeAddr, regTemp); err != nil {
		t.Fatalf("Unwatch: %v", err)
	}
	mu.Lock()
	baseline := watches
	mu.Unlock()
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	after := watches
	mu.Unlock()
	if after != baseline {
		t.Fatalf("WATCH refreshes continued after Unwatch: %d -> %d", baseline, after)
	}
}

// TestEngine_OfflineNodeReportsNull verifies that when a watched node stops
// answering refreshes (e.g. it is powered off), the engine delivers a NULL
// update to the watcher after LivenessMisses consecutive unanswered refreshes,
// so a stale value is not reported indefinitely.
func TestEngine_OfflineNodeReportsNull(t *testing.T) {
	c, err := protocol.NewCodec(testKey)
	if err != nil {
		t.Fatal(err)
	}
	e := New(Options{
		HubAddr:         hubAddr,
		Timeout:         20 * time.Millisecond,
		Retries:         1,
		RefreshInterval: 15 * time.Millisecond,
		LivenessMisses:  2,
	})
	f := newFakeRadio()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := e.AddRadio(ctx, testChannel, f); err != nil {
		t.Fatal(err)
	}
	n := node.NewNode("t", testChannel, &node.Descriptor{},
		node.Identity{Address: nodeAddr, Key: testKey})
	if err := e.AddNode(n); err != nil {
		t.Fatal(err)
	}

	// The simulated node answers while alive, then goes silent (powered off).
	var smu sync.Mutex
	alive := true
	go func() {
		for req := range f.sent {
			smu.Lock()
			up := alive
			smu.Unlock()
			if !up {
				continue // node is off: no reply
			}
			_, _, _, reg, _, derr := c.Decode(req[:])
			if derr != nil {
				t.Errorf("node decode: %v", derr)
				continue
			}
			var resp [PacketLen]byte
			c.Encode(resp[:], nodeAddr, protocol.TypeIS, 0, reg, 11)
			f.recv <- resp
		}
	}()

	var cmu sync.Mutex
	var updates []Update
	if err := e.Watch(context.Background(), nodeAddr, regTemp, func(u Update) {
		cmu.Lock()
		updates = append(updates, u)
		cmu.Unlock()
	}); err != nil {
		t.Fatalf("Watch: %v", err)
	}

	go e.Run(ctx)

	// Let a few refreshes succeed, then power the node off.
	time.Sleep(45 * time.Millisecond)
	smu.Lock()
	alive = false
	smu.Unlock()

	// Expect a NULL update once the refreshes go unanswered past the threshold.
	deadline := time.After(2 * time.Second)
	for {
		cmu.Lock()
		var sawNull bool
		for _, u := range updates {
			if u.Null {
				sawNull = true
			}
		}
		cmu.Unlock()
		if sawNull {
			break
		}
		select {
		case <-deadline:
			t.Fatal("expected a NULL update after node went offline")
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// TestEngine_AcksSpontaneousPush checks the hub acknowledges a received
// spontaneous push (IS with the PUSH flag, protocol/README.md §8.3) so the node stops
// retransmitting it.
func TestEngine_AcksSpontaneousPush(t *testing.T) {
	_, f, c, cancel := newEngine(t)
	defer cancel()

	var pkt [PacketLen]byte
	c.Encode(pkt[:], nodeAddr, protocol.TypeIS, protocol.FlagPush, regTemp, 99)
	f.recv <- pkt

	select {
	case got := <-f.sent:
		_, typ, _, reg, _, err := c.Decode(got[:])
		if err != nil {
			t.Fatalf("decode ACK: %v", err)
		}
		if typ != protocol.TypeACK {
			t.Fatalf("reply type = %#x, want ACK", typ)
		}
		if reg != regTemp {
			t.Fatalf("ACK reg = %#x, want %#x", reg, regTemp)
		}
	case <-time.After(time.Second):
		t.Fatal("engine did not ACK the spontaneous push")
	}
}

// TestEngine_DoesNotAckSolicitedIS checks the hub does not ACK a plain IS (PUSH
// clear): solicited replies are recovered by request retransmission, and a
// spurious ACK would waste a transmit window.
func TestEngine_DoesNotAckSolicitedIS(t *testing.T) {
	_, f, c, cancel := newEngine(t)
	defer cancel()

	var pkt [PacketLen]byte
	c.Encode(pkt[:], nodeAddr, protocol.TypeIS, 0, regTemp, 7) // no PUSH flag
	f.recv <- pkt

	select {
	case got := <-f.sent:
		_, typ, _, _, _, _ := c.Decode(got[:])
		t.Fatalf("engine transmitted type %#x for a solicited IS, want nothing", typ)
	case <-time.After(150 * time.Millisecond):
		// No transmit: correct.
	}
}
