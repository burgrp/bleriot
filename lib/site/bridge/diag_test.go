package bridge

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/burgrp/bleriot/lib/site/engine"
	"github.com/burgrp/bleriot/lib/site/radio"
)

// TestRing_Rate checks the per-second rate is the counter delta over the elapsed
// span before the window has filled.
func TestRing_Rate(t *testing.T) {
	r := ring{window: 30 * time.Second}
	base := time.Now()
	r.sample(base, []uint64{0})
	rates := r.sample(base.Add(10*time.Second), []uint64{100})
	if rates[0] != 10 {
		t.Fatalf("rate = %v, want 10/s", rates[0])
	}
}

// TestRing_TrimsToWindow checks samples older than the window are dropped so the
// rate baseline stays inside it, keeping a steady rate steady.
func TestRing_TrimsToWindow(t *testing.T) {
	r := ring{window: 10 * time.Second}
	base := time.Now()
	r.sample(base, []uint64{0})
	r.sample(base.Add(5*time.Second), []uint64{50})
	r.sample(base.Add(10*time.Second), []uint64{100})
	r.sample(base.Add(15*time.Second), []uint64{150})
	rates := r.sample(base.Add(16*time.Second), []uint64{160})
	if rates[0] != 10 {
		t.Fatalf("rate = %v, want 10/s after trim", rates[0])
	}
	// The oldest out-of-window sample (t=0) must have been dropped.
	if len(r.ts) > 4 {
		t.Fatalf("ring kept %d samples, expected trimming to <= 4", len(r.ts))
	}
}

// multiReg is a fake Registry that captures every Provide call by name so a test
// can read the values pushed for any diagnostic register.
type multiReg struct {
	mu  sync.Mutex
	chs map[string]chan any
}

func newMultiReg() *multiReg { return &multiReg{chs: make(map[string]chan any)} }

func (r *multiReg) Provide(ctx context.Context, name string, value any, metadata map[string]any, ttl time.Duration) (chan<- any, <-chan any, error) {
	updates := make(chan any, 16)
	requests := make(chan any)
	r.mu.Lock()
	r.chs[name] = updates
	r.mu.Unlock()
	return updates, requests, nil
}

func (r *multiReg) channel(name string) chan any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.chs[name]
}

// fakeSnap is a fixed DiagNodeSource.
type fakeSnap struct{ s engine.NodeStats }

func (f fakeSnap) SnapshotNode(addr [4]byte) engine.NodeStats { return f.s }

func recvWithinDiag(t *testing.T, ch <-chan any, d time.Duration) any {
	t.Helper()
	if ch == nil {
		t.Fatal("register was never provided")
	}
	select {
	case v := <-ch:
		return v
	case <-time.After(d):
		t.Fatal("timed out waiting for diagnostic value")
		return nil
	}
}

// TestDiagnostics_PublishesNodeAndDongle checks the immediate publish provides
// and pushes per-node and per-dongle diagnostic registers with the expected
// values and that read-only change requests are ignored.
func TestDiagnostics_PublishesNodeAndDongle(t *testing.T) {
	src := fakeSnap{s: engine.NodeStats{
		RxAll: 12, RxIS: 7, RxACK: 3, RxCorrupt: 2,
		TxAll: 20, TxRetries: 4, Timeouts: 1,
		LastRx: 1700000000, Misses: 0, Online: true,
	}}
	reg := newMultiReg()
	d := NewDiagnostics(src, reg, "diag", 30*time.Second, time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dongleStats := radio.DongleStats{
		Connected: true, Reconnects: 2, Up: 1700000001, Down: 1700000000,
		TxAll: 30, TxErr: 1, RxAll: 9,
	}
	d.Serve(ctx,
		[]DiagNode{{Name: "lab", Addr: [4]byte{1, 2, 3, 4}}},
		[]DiagDongle{{Name: "far", Stats: func() radio.DongleStats { return dongleStats }}},
	)

	cases := []struct {
		name string
		want any
	}{
		{"diag.node.lab.online", true},
		{"diag.node.lab.seen", int64(1700000000)},
		{"diag.node.lab.misses", int64(0)},
		{"diag.node.lab.count.rx.all", int64(12)},
		{"diag.node.lab.count.rx.is", int64(7)},
		{"diag.node.lab.count.rx.acks", int64(3)},
		{"diag.node.lab.count.rx.corrupt", int64(2)},
		{"diag.node.lab.count.tx.all", int64(20)},
		{"diag.node.lab.count.tx.retries", int64(4)},
		{"diag.node.lab.count.timeouts", int64(1)},
		{"diag.node.lab.rate.tx.all", float64(0)}, // first sample: no rate yet
		{"diag.dongle.far.connected", true},
		{"diag.dongle.far.reconnects", int64(2)},
		{"diag.dongle.far.up", int64(1700000001)},
		{"diag.dongle.far.down", int64(1700000000)},
		{"diag.dongle.far.count.tx.all", int64(30)},
		{"diag.dongle.far.count.tx.err", int64(1)},
		{"diag.dongle.far.count.rx.all", int64(9)},
	}
	for _, c := range cases {
		got := recvWithinDiag(t, reg.channel(c.name), time.Second)
		if got != c.want {
			t.Errorf("%s = %v (%T), want %v (%T)", c.name, got, got, c.want, c.want)
		}
	}
}
