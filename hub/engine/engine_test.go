package engine

import (
	"context"
	"sync"
	"testing"
	"time"

	"bleriot"
	"hub/node"
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
	sent chan [PacketLen]byte
	recv chan [PacketLen]byte
	drop int // drop the first n Send calls (to exercise retries)
	mu   sync.Mutex
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

// simulateNode reads one request, decodes it, and replies with an IS packet.
// reply transforms the request value into the response value.
func simulateNode(t *testing.T, f *fakeRadio, c bleriot.Codec, reply func(typ byte, reg uint16, val int32) (int32, bool)) {
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
			flags := byte(0)
			if null {
				flags = bleriot.FlagNULL
			}
			c.Encode(resp[:], nodeAddr, bleriot.TypeIS, flags, reg, rv)
			f.recv <- resp
		}
	}()
}

func newEngine(t *testing.T) (*Engine, *fakeRadio, bleriot.Codec, context.CancelFunc) {
	t.Helper()
	c, err := bleriot.NewCodec(testKey)
	if err != nil {
		t.Fatal(err)
	}
	e := New(Options{HubAddr: hubAddr, Timeout: 50 * time.Millisecond, Retries: 3})
	f := newFakeRadio()
	ctx, cancel := context.WithCancel(context.Background())
	e.AddRadio(ctx, testChannel, f)

	n := node.NewNode(
		"t",
		&node.Descriptor{Channel: testChannel},
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

func TestEngine_Set(t *testing.T) {
	e, f, c, cancel := newEngine(t)
	defer cancel()
	// Node clamps to a max of 100.
	simulateNode(t, f, c, func(typ byte, reg uint16, val int32) (int32, bool) {
		if typ != bleriot.TypeSET {
			t.Errorf("expected SET, got %d", typ)
		}
		if val > 100 {
			val = 100
		}
		return val, false
	})

	u, err := e.Set(context.Background(), nodeAddr, regTemp, 250)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if u.Value != 100 {
		t.Fatalf("Set clamp = %d, want 100", u.Value)
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
	c.Encode(push[:], nodeAddr, bleriot.TypeIS, 0, regTemp, 22)
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
	c, err := bleriot.NewCodec(testKey)
	if err != nil {
		t.Fatal(err)
	}
	// Short refresh interval so the test runs quickly.
	e := New(Options{HubAddr: hubAddr, Timeout: 50 * time.Millisecond, Retries: 3, RefreshInterval: 20 * time.Millisecond})
	f := newFakeRadio()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.AddRadio(ctx, testChannel, f)
	n := node.NewNode(
		"t",
		&node.Descriptor{Channel: testChannel},
		node.Identity{Address: nodeAddr, Key: testKey},
	)
	if err := e.AddNode(n); err != nil {
		t.Fatal(err)
	}

	// Count WATCH requests the node sees.
	var mu sync.Mutex
	var watches int
	simulateNode(t, f, c, func(typ byte, reg uint16, val int32) (int32, bool) {
		if typ == bleriot.TypeWATCH && val == 1 {
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
