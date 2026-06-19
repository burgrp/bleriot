package bridge

import (
	"context"
	"sync"
	"testing"
	"time"

	"site/engine"
	"site/node"
)

var nodeAddr = [4]byte{0xCC, 0xA0, 0x00, 0x02}

// fakeTx is a fake Transactor. It records Set calls and exposes the registered
// watch callback so tests can drive pushes.
type fakeTx struct {
	mu       sync.Mutex
	getU     engine.Update
	getErr   error
	watchCb  engine.Callback
	sets     []int32
	setNulls int
}

func (f *fakeTx) Get(ctx context.Context, addr [4]byte, reg uint16) (engine.Update, error) {
	return f.getU, f.getErr
}

func (f *fakeTx) Set(ctx context.Context, addr [4]byte, reg uint16, value int32) error {
	f.mu.Lock()
	f.sets = append(f.sets, value)
	cb := f.watchCb
	f.mu.Unlock()
	// Emulate the node's value change flowing back through the subscription.
	if cb != nil {
		cb(engine.Update{Value: value})
	}
	return nil
}

func (f *fakeTx) SetNull(ctx context.Context, addr [4]byte, reg uint16) error {
	f.mu.Lock()
	f.setNulls++
	cb := f.watchCb
	f.mu.Unlock()
	// Emulate the node clearing the register and pushing a NULL value back.
	if cb != nil {
		cb(engine.Update{Null: true})
	}
	return nil
}

func (f *fakeTx) Watch(ctx context.Context, addr [4]byte, reg uint16, cb engine.Callback) error {
	f.mu.Lock()
	f.watchCb = cb
	f.mu.Unlock()
	return nil
}

func (f *fakeTx) push(u engine.Update) {
	f.mu.Lock()
	cb := f.watchCb
	f.mu.Unlock()
	if cb != nil {
		cb(u)
	}
}

// fakeReg is a fake Registry capturing the Provide call and exposing channels.
type fakeReg struct {
	name     string
	initial  any
	metadata map[string]any
	updates  chan any
	requests chan any
}

func newFakeReg() *fakeReg {
	return &fakeReg{updates: make(chan any, 16), requests: make(chan any, 16)}
}

func (r *fakeReg) Provide(ctx context.Context, name string, value any, metadata map[string]any, ttl time.Duration) (chan<- any, <-chan any, error) {
	r.name = name
	r.initial = value
	r.metadata = metadata
	return r.updates, r.requests, nil
}

func floatReg() *node.Register {
	return &node.Register{
		ID: 0x1234, Name: "outdoor.temperature",
		Type: node.TypeFloat, Multiplier: 1, Divider: 100,
		Metadata: map[string]string{"unit": "celsius"},
	}
}

func serve(t *testing.T, tx Transactor, reg Registry, r *node.Register) context.CancelFunc {
	t.Helper()
	b := New(tx, reg, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	n := node.NewNode("test", 10, &node.Descriptor{Registers: []node.Register{*r}}, node.Identity{Address: nodeAddr})
	b.ServeNode(ctx, n)
	return cancel
}

func recvWithin(t *testing.T, ch <-chan any, d time.Duration) any {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(d):
		t.Fatal("timed out waiting for registry update")
		return nil
	}
}

func TestBridge_SeedsInitialValue(t *testing.T) {
	tx := &fakeTx{getU: engine.Update{Value: 1234}}
	reg := newFakeReg()
	cancel := serve(t, tx, reg, floatReg())
	defer cancel()

	// Give ServeNode goroutines a moment to call Provide.
	deadline := time.After(time.Second)
	for reg.name == "" {
		select {
		case <-deadline:
			t.Fatal("Provide was not called")
		case <-time.After(2 * time.Millisecond):
		}
	}
	if reg.name != "test.outdoor.temperature" {
		t.Errorf("provided name = %q", reg.name)
	}
	if reg.initial != 12.34 {
		t.Errorf("initial value = %v, want 12.34", reg.initial)
	}
	if reg.metadata["unit"] != "celsius" || reg.metadata["type"] != "float" || reg.metadata["device"] != "test" {
		t.Errorf("metadata = %v", reg.metadata)
	}
}

func TestBridge_PushUpdatesRegistry(t *testing.T) {
	tx := &fakeTx{getU: engine.Update{Value: 0}}
	reg := newFakeReg()
	cancel := serve(t, tx, reg, floatReg())
	defer cancel()

	// Wait until the watch callback is registered.
	deadline := time.After(time.Second)
	for {
		tx.mu.Lock()
		ready := tx.watchCb != nil
		tx.mu.Unlock()
		if ready {
			break
		}
		select {
		case <-deadline:
			t.Fatal("watch never registered")
		case <-time.After(2 * time.Millisecond):
		}
	}

	tx.push(engine.Update{Value: 2550}) // 25.50 °C
	if got := recvWithin(t, reg.updates, time.Second); got != 25.5 {
		t.Errorf("update = %v, want 25.5", got)
	}
}

func TestBridge_ChangeRequestTriggersSet(t *testing.T) {
	tx := &fakeTx{getU: engine.Update{Value: 0}}
	reg := newFakeReg()
	cancel := serve(t, tx, reg, floatReg())
	defer cancel()

	// Wait for watch registration so the Set's emulated IS has a callback.
	deadline := time.After(time.Second)
	for {
		tx.mu.Lock()
		ready := tx.watchCb != nil
		tx.mu.Unlock()
		if ready {
			break
		}
		select {
		case <-deadline:
			t.Fatal("watch never registered")
		case <-time.After(2 * time.Millisecond):
		}
	}

	// Consumer requests 18.0 °C -> wire 1800.
	reg.requests <- 18.0

	if got := recvWithin(t, reg.updates, time.Second); got != 18.0 {
		t.Errorf("reflected update = %v, want 18.0", got)
	}
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if len(tx.sets) != 1 || tx.sets[0] != 1800 {
		t.Errorf("sets = %v, want [1800]", tx.sets)
	}
}

func TestBridge_NilChangeRequestTriggersSetNull(t *testing.T) {
	tx := &fakeTx{getU: engine.Update{Value: 0}}
	reg := newFakeReg()
	cancel := serve(t, tx, reg, floatReg())
	defer cancel()

	// Wait for watch registration so the SetNull's emulated push has a callback.
	deadline := time.After(time.Second)
	for {
		tx.mu.Lock()
		ready := tx.watchCb != nil
		tx.mu.Unlock()
		if ready {
			break
		}
		select {
		case <-deadline:
			t.Fatal("watch never registered")
		case <-time.After(2 * time.Millisecond):
		}
	}

	// A nil change request clears the register.
	reg.requests <- nil

	if got := recvWithin(t, reg.updates, time.Second); got != nil {
		t.Errorf("reflected update = %v, want nil", got)
	}
	tx.mu.Lock()
	defer tx.mu.Unlock()
	if tx.setNulls != 1 {
		t.Errorf("setNulls = %d, want 1", tx.setNulls)
	}
	if len(tx.sets) != 0 {
		t.Errorf("sets = %v, want none", tx.sets)
	}
}

func TestBridge_NullPushMapsToNil(t *testing.T) {
	tx := &fakeTx{getErr: engine.ErrTimeout} // no seed
	reg := newFakeReg()
	cancel := serve(t, tx, reg, floatReg())
	defer cancel()

	deadline := time.After(time.Second)
	for {
		tx.mu.Lock()
		ready := tx.watchCb != nil
		tx.mu.Unlock()
		if ready {
			break
		}
		select {
		case <-deadline:
			t.Fatal("watch never registered")
		case <-time.After(2 * time.Millisecond):
		}
	}

	tx.push(engine.Update{Null: true})
	if got := recvWithin(t, reg.updates, time.Second); got != nil {
		t.Errorf("null update = %v, want nil", got)
	}
}
