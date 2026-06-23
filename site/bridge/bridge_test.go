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

// testRegID is the wire ID of the single register served by these tests; the
// fake Transactor routes pushes to it.
const testRegID uint16 = 0x1234

// fakeTx is a fake Transactor. It records Set calls and exposes the registered
// watch-all callback so tests can drive pushes.
type fakeTx struct {
	mu         sync.Mutex
	getU       engine.Update
	getErr     error
	getCalls   int
	watchAllCb engine.AllCallback
	sets       []int32
	setNulls   int
}

func (f *fakeTx) Get(ctx context.Context, addr [4]byte, reg uint16) (engine.Update, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getCalls++
	return f.getU, f.getErr
}

// setGet replaces the value and error the fake Get returns, under the lock.
func (f *fakeTx) setGet(u engine.Update, err error) {
	f.mu.Lock()
	f.getU, f.getErr = u, err
	f.mu.Unlock()
}

// gets returns how many times Get has been called.
func (f *fakeTx) gets() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getCalls
}

func (f *fakeTx) Set(ctx context.Context, addr [4]byte, reg uint16, value int32) error {
	f.mu.Lock()
	f.sets = append(f.sets, value)
	cb := f.watchAllCb
	f.mu.Unlock()
	// Emulate the node's value change flowing back through the subscription.
	if cb != nil {
		cb(reg, engine.Update{Value: value})
	}
	return nil
}

func (f *fakeTx) SetNull(ctx context.Context, addr [4]byte, reg uint16) error {
	f.mu.Lock()
	f.setNulls++
	cb := f.watchAllCb
	f.mu.Unlock()
	// Emulate the node clearing the register and pushing a NULL value back.
	if cb != nil {
		cb(reg, engine.Update{Null: true})
	}
	return nil
}

func (f *fakeTx) WatchAll(ctx context.Context, addr [4]byte, cb engine.AllCallback) error {
	f.mu.Lock()
	f.watchAllCb = cb
	f.mu.Unlock()
	return nil
}

func (f *fakeTx) push(u engine.Update) {
	f.mu.Lock()
	cb := f.watchAllCb
	f.mu.Unlock()
	if cb != nil {
		cb(testRegID, u)
	}
}

// pushAll drives the watch-all offline signal (reg == engine.RegAll), as the
// engine raises it when a node goes offline (§10) and every register is nulled.
func (f *fakeTx) pushAll(u engine.Update) {
	f.mu.Lock()
	cb := f.watchAllCb
	f.mu.Unlock()
	if cb != nil {
		cb(engine.RegAll, u)
	}
}

// ready reports whether the watch-all callback has been registered.
func (f *fakeTx) ready() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.watchAllCb != nil
}

// fakeReg is a fake Registry capturing the Provide call and exposing channels.
type fakeReg struct {
	mu       sync.Mutex
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
	r.mu.Lock()
	r.name = name
	r.initial = value
	r.metadata = metadata
	r.mu.Unlock()
	return r.updates, r.requests, nil
}

// provided returns the captured arguments of the most recent Provide call under
// the lock, so a test can read them without racing the goroutine that calls it.
func (r *fakeReg) provided() (name string, initial any, metadata map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.name, r.initial, r.metadata
}

func floatReg() *node.Register {
	return &node.Register{
		ID: testRegID, Name: "outdoor.temperature",
		Type: node.TypeFloat, Multiplier: 1, Divider: 100,
		Metadata: map[string]string{"unit": "celsius"},
	}
}

func serve(t *testing.T, tx Transactor, reg Registry, r *node.Register) context.CancelFunc {
	t.Helper()
	b := New(tx, reg, time.Second)
	b.seedRetry = 5 * time.Millisecond // fast retries so reseed tests do not wait
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

// waitReady blocks until the bridge has registered its watch-all callback.
func waitReady(t *testing.T, tx *fakeTx) {
	t.Helper()
	deadline := time.After(time.Second)
	for !tx.ready() {
		select {
		case <-deadline:
			t.Fatal("watch-all never registered")
		case <-time.After(2 * time.Millisecond):
		}
	}
}

func TestBridge_SeedsInitialValue(t *testing.T) {
	tx := &fakeTx{getU: engine.Update{Value: 1234}}
	reg := newFakeReg()
	cancel := serve(t, tx, reg, floatReg())
	defer cancel()

	// Give ServeNode goroutines a moment to call Provide.
	deadline := time.After(time.Second)
	for {
		name, _, _ := reg.provided()
		if name != "" {
			break
		}
		select {
		case <-deadline:
			t.Fatal("Provide was not called")
		case <-time.After(2 * time.Millisecond):
		}
	}
	name, initial, md := reg.provided()
	if name != "test.outdoor.temperature" {
		t.Errorf("provided name = %q", name)
	}
	if initial != 12.34 {
		t.Errorf("initial value = %v, want 12.34", initial)
	}
	if md["unit"] != "celsius" || md["type"] != "float" || md["device"] != "test" {
		t.Errorf("metadata = %v", md)
	}
}

func TestBridge_PushUpdatesRegistry(t *testing.T) {
	tx := &fakeTx{getU: engine.Update{Value: 0}}
	reg := newFakeReg()
	cancel := serve(t, tx, reg, floatReg())
	defer cancel()

	// Wait until the watch-all callback is registered.
	waitReady(t, tx)

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
	waitReady(t, tx)

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
	waitReady(t, tx)

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

	waitReady(t, tx)

	tx.push(engine.Update{Null: true})
	if got := recvWithin(t, reg.updates, time.Second); got != nil {
		t.Errorf("null update = %v, want nil", got)
	}
}

// TestBridge_RetriesSeedingUntilNodeAnswers verifies that when the initial
// seeding GET fails (the node is unreachable at startup, e.g. the dongle is
// still enumerating), the bridge keeps retrying and seeds the register once the
// node answers — rather than leaving it null until its next spontaneous change.
func TestBridge_RetriesSeedingUntilNodeAnswers(t *testing.T) {
	tx := &fakeTx{getErr: engine.ErrTimeout} // node unreachable at startup
	reg := newFakeReg()
	cancel := serve(t, tx, reg, floatReg())
	defer cancel()

	// Wait until the retry loop has attempted at least one more GET beyond the
	// initial failed seed, then let the node start answering.
	deadline := time.After(time.Second)
	for tx.gets() < 2 {
		select {
		case <-deadline:
			t.Fatal("seeding GET was not retried")
		case <-time.After(2 * time.Millisecond):
		}
	}
	tx.setGet(engine.Update{Value: 2000}, nil) // 20.00 °C

	if got := recvWithin(t, reg.updates, time.Second); got != 20.0 {
		t.Errorf("delayed seed = %v, want 20.0", got)
	}
}

// TestBridge_StopsSeedingAfterPush verifies a push counts as a seed: once a
// value arrives via the subscription, the retry loop stops issuing GETs even if
// the node never answered a GET.
func TestBridge_StopsSeedingAfterPush(t *testing.T) {
	tx := &fakeTx{getErr: engine.ErrTimeout} // GET keeps failing
	reg := newFakeReg()
	cancel := serve(t, tx, reg, floatReg())
	defer cancel()

	waitReady(t, tx)
	tx.push(engine.Update{Value: 1000}) // a push seeds the value
	if got := recvWithin(t, reg.updates, time.Second); got != 10.0 {
		t.Errorf("push seed = %v, want 10.0", got)
	}

	// After the push, the retry loop must stop: record the GET count and confirm
	// it no longer grows.
	settled := tx.gets()
	time.Sleep(40 * time.Millisecond) // several retry intervals
	if grew := tx.gets() - settled; grew > 0 {
		t.Errorf("seeding GETs continued after a push seeded the value: +%d", grew)
	}
}

// TestBridge_ReseedsAfterReconnect verifies the reconnect path: when the node is
// signalled offline (the dongle disconnects, §10) every register is nulled, and
// once the node is reachable again its value is re-fetched and republished
// without waiting for the register's next spontaneous change — a watch-all
// refresh (§8.3) only draws an ACK, so nothing else would re-seed it.
func TestBridge_ReseedsAfterReconnect(t *testing.T) {
	tx := &fakeTx{getU: engine.Update{Value: 4200}} // seeded to 42.00 °C
	reg := newFakeReg()
	cancel := serve(t, tx, reg, floatReg())
	defer cancel()
	waitReady(t, tx)

	// The node drops: make GETs fail first so the post-offline reseed loop cannot
	// recover the stale value, then raise the offline signal.
	tx.setGet(engine.Update{}, engine.ErrTimeout)
	tx.pushAll(engine.Update{Null: true})
	if got := recvWithin(t, reg.updates, time.Second); got != nil {
		t.Errorf("offline update = %v, want nil", got)
	}
	time.Sleep(20 * time.Millisecond) // let a few reseed GETs fail while offline

	// The dongle returns and the node answers again with its current value.
	tx.setGet(engine.Update{Value: 4300}, nil) // 43.00 °C
	if got := recvWithin(t, reg.updates, time.Second); got != 43.0 {
		t.Errorf("reseeded update after reconnect = %v, want 43.0", got)
	}
}

// TestBridge_BacksOffUnansweredSeed verifies the seeding GET backs off when a
// register is never answered (e.g. declared in the inventory but not implemented
// on the node), so it cannot saturate the shared radio. With a 5 ms base and
// exponential backoff, the GET count over a fixed window stays far below the
// ~linear rate an un-throttled retry would produce, while still making progress.
func TestBridge_BacksOffUnansweredSeed(t *testing.T) {
	tx := &fakeTx{getErr: engine.ErrTimeout} // node never answers this register
	reg := newFakeReg()
	cancel := serve(t, tx, reg, floatReg())
	defer cancel()
	waitReady(t, tx)

	time.Sleep(300 * time.Millisecond) // many base intervals
	got := tx.gets()
	// Un-throttled at the 5 ms base this window would yield ~60 GETs; backoff
	// (5,10,20,40,80,160,… ms) keeps it to a handful. Bound generously to absorb
	// jitter and scheduling slack while still proving the rate is throttled.
	if got > 15 {
		t.Errorf("seeding GETs not backed off: %d in 300ms (want <= 15)", got)
	}
	if got < 3 {
		t.Errorf("seeding made no progress: only %d GETs (expected retries)", got)
	}
}
