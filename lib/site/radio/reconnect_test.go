package radio

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// fakeDongle is a controllable Dongle for testing the Reconnecting supervisor.
type fakeDongle struct {
	mu         sync.Mutex
	sendErr    error // returned by Send when non-nil
	receiveErr error // returned by Receive when non-nil
	rx         bool  // Receive returns one packet when true
	closed     bool
	sendN      int
	receiveN   int
}

func (f *fakeDongle) Send(dst [4]byte, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sendN++
	return f.sendErr
}

func (f *fakeDongle) Receive(buf []byte) (int, bool) {
	n, ok, _ := f.ReceiveWithError(buf)
	return n, ok
}

func (f *fakeDongle) ReceiveWithError(buf []byte) (int, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.receiveN++
	if f.receiveErr != nil {
		return 0, false, f.receiveErr
	}
	if f.rx {
		return len(buf), true, nil
	}
	return 0, false, nil
}

func (f *fakeDongle) ReplyGuard() time.Duration { return 0 }

func (f *fakeDongle) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeDongle) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// quietLogger discards log output so tests stay silent.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met within deadline")
}

func TestReconnecting_OfflineUntilOpened(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var dev *fakeDongle
	var fail bool // when true, open reports an error (device absent)
	fail = true

	open := func() (Dongle, error) {
		mu.Lock()
		defer mu.Unlock()
		if fail {
			return nil, errors.New("absent")
		}
		dev = &fakeDongle{}
		return dev, nil
	}

	r := NewReconnecting(ctx, open, 7*time.Millisecond, 5*time.Millisecond, quietLogger())
	defer r.Close()

	// Guard is known before any device exists, so the hub can start offline.
	if r.ReplyGuard() != 7*time.Millisecond {
		t.Fatalf("ReplyGuard = %v, want 7ms", r.ReplyGuard())
	}
	// While offline, Send reports ErrOffline and Receive yields nothing.
	if err := r.Send([4]byte{}, nil); !errors.Is(err, ErrOffline) {
		t.Fatalf("Send while offline = %v, want ErrOffline", err)
	}
	if _, ok := r.Receive(make([]byte, 4)); ok {
		t.Fatal("Receive while offline returned a packet")
	}

	// The device appears; the supervisor connects within a couple of backoffs.
	mu.Lock()
	fail = false
	mu.Unlock()

	waitFor(t, func() bool {
		return r.Send([4]byte{}, nil) == nil
	})
}

func TestReconnecting_ReopensAfterSendFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var devs []*fakeDongle
	open := func() (Dongle, error) {
		mu.Lock()
		defer mu.Unlock()
		d := &fakeDongle{}
		devs = append(devs, d)
		return d, nil
	}

	r := NewReconnecting(ctx, open, 0, 5*time.Millisecond, quietLogger())
	defer r.Close()

	// Wait for the first device to connect.
	waitFor(t, func() bool { return r.Send([4]byte{}, nil) == nil })

	mu.Lock()
	first := devs[0]
	mu.Unlock()

	// Make the live device fail its next Send: it should be dropped and closed.
	first.mu.Lock()
	first.sendErr = errors.New("disconnected")
	first.mu.Unlock()

	if err := r.Send([4]byte{}, nil); err == nil {
		t.Fatal("Send to failing device returned nil")
	}
	waitFor(t, func() bool { return first.isClosed() })

	// The supervisor reopens a fresh device and Send succeeds again.
	waitFor(t, func() bool {
		mu.Lock()
		n := len(devs)
		mu.Unlock()
		return n >= 2 && r.Send([4]byte{}, nil) == nil
	})
}

func TestReconnecting_ReopensAfterReceiveFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var devs []*fakeDongle
	open := func() (Dongle, error) {
		mu.Lock()
		defer mu.Unlock()
		d := &fakeDongle{}
		devs = append(devs, d)
		return d, nil
	}

	r := NewReconnecting(ctx, open, 0, 5*time.Millisecond, quietLogger())
	defer r.Close()
	waitFor(t, func() bool { return r.Send([4]byte{}, nil) == nil })

	mu.Lock()
	first := devs[0]
	mu.Unlock()
	first.mu.Lock()
	first.receiveErr = errors.New("disconnected")
	first.mu.Unlock()

	r.Receive(make([]byte, 4))
	waitFor(t, func() bool { return first.isClosed() })
	waitFor(t, func() bool {
		mu.Lock()
		n := len(devs)
		mu.Unlock()
		return n >= 2 && r.Send([4]byte{}, nil) == nil
	})
}

func TestReconnecting_CloseStopsSupervisor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var dev *fakeDongle
	open := func() (Dongle, error) {
		dev = &fakeDongle{}
		return dev, nil
	}
	r := NewReconnecting(ctx, open, 0, 5*time.Millisecond, quietLogger())
	waitFor(t, func() bool { return r.Send([4]byte{}, nil) == nil })

	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !dev.isClosed() {
		t.Fatal("Close did not close the live device")
	}
	// After Close the dongle stays offline.
	if err := r.Send([4]byte{}, nil); !errors.Is(err, ErrOffline) {
		t.Fatalf("Send after Close = %v, want ErrOffline", err)
	}
}
