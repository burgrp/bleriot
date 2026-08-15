package radio

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// ErrOffline is returned by a Reconnecting dongle's Send while no physical device
// is connected. The engine treats it like any other send failure: the
// transaction fails and is retried, and a watch refresh that hits it counts
// toward the node's liveness, so a register served by an offline dongle is
// reported NULL until the device returns.
var ErrOffline = errors.New("radio: dongle offline")

// DefaultReconnectBackoff is the wait between reconnection attempts when no
// backoff is given to NewReconnecting.
const DefaultReconnectBackoff = 2 * time.Second

// Reconnecting wraps a dongle opener and keeps a device connected for the hub's
// lifetime, transparently reopening it after a disconnect. It satisfies Dongle
// with a stable identity, so the engine holds one radio per channel regardless
// of the physical device being unplugged and replugged.
//
// The reply guard (lib/README.md §6) is fixed at construction: it depends only on
// the channel's spreading factor, not the hardware, so the engine can validate
// and use it before any device is present. This lets the hub start with every
// dongle disconnected.
//
// A disconnect is detected from a Send or Receive failure. The failed device is
// closed and the supervisor reopens it; Receive simply yields no packets while
// offline. A selector that re-resolves the device each open (e.g. a USB serial
// rather than a fixed /dev/hidraw path) reconnects even when the device returns
// on a different node.
type Reconnecting struct {
	open    func() (Dongle, error)
	guard   time.Duration
	backoff time.Duration
	log     *slog.Logger

	wake chan struct{} // buffered(1): nudges the supervisor to reopen after a drop

	mu     sync.Mutex
	cur    Dongle // the live device, or nil while offline
	closed bool

	// Diagnostic counters, updated lock-free. connected mirrors cur!=nil for
	// snapshotting without taking mu; reconnects counts reopens after the first
	// connect; lastConnect/lastDisconnect are the unix-seconds times of the most
	// recent open and the most recent disconnect.
	connected      atomic.Bool
	reconnects     atomic.Uint64
	lastConnect    atomic.Int64
	lastDisconnect atomic.Int64
	txAll          atomic.Uint64 // Send calls
	txErr          atomic.Uint64 // Send calls that failed (including while offline)
	rxAll          atomic.Uint64 // packets actually received
}

// DongleStats is a point-in-time snapshot of a Reconnecting dongle's connection
// state and cumulative traffic counters, as published by the diagnostics bridge.
type DongleStats struct {
	// Connected reports whether a physical device is currently open.
	Connected bool
	// Reconnects counts how many times the device has been reopened after the
	// first successful connect (a measure of link instability).
	Reconnects uint64
	// Up is the unix-seconds time of the most recent successful open, or 0 if the
	// device has never connected. While Connected it marks the start of the
	// current uptime.
	Up int64
	// Down is the unix-seconds time of the most recent disconnect, or 0 if the
	// device has never dropped. While offline it marks the start of the current
	// outage; together with Up it bounds the last completed session.
	Down  int64
	TxAll uint64 // transmit attempts
	TxErr uint64 // failed transmit attempts (including those made while offline)
	RxAll uint64 // packets received
}

// Stats returns a snapshot of the dongle's connection state and traffic counters.
func (r *Reconnecting) Stats() DongleStats {
	return DongleStats{
		Connected:  r.connected.Load(),
		Reconnects: r.reconnects.Load(),
		Up:         r.lastConnect.Load(),
		Down:       r.lastDisconnect.Load(),
		TxAll:      r.txAll.Load(),
		TxErr:      r.txErr.Load(),
		RxAll:      r.rxAll.Load(),
	}
}

// NewReconnecting creates a Reconnecting dongle and starts its supervisor, which
// keeps a device connected until ctx is cancelled or Close is called. open is
// called to (re)open the physical device; guard is the device's reply turnaround
// (lib/README.md §6); backoff is the wait between reconnection attempts (defaults
// to DefaultReconnectBackoff). A nil logger falls back to slog.Default.
func NewReconnecting(ctx context.Context, open func() (Dongle, error), guard, backoff time.Duration, log *slog.Logger) *Reconnecting {
	if backoff <= 0 {
		backoff = DefaultReconnectBackoff
	}
	if log == nil {
		log = slog.Default()
	}
	r := &Reconnecting{
		open:    open,
		guard:   guard,
		backoff: backoff,
		log:     log,
		wake:    make(chan struct{}, 1),
	}
	go r.supervise(ctx)
	return r
}

// ReplyGuard reports the reply turnaround guard (lib/README.md §6), known up front
// from the channel's spreading factor and independent of the live device.
func (r *Reconnecting) ReplyGuard() time.Duration { return r.guard }

// Send transmits via the live device, or reports ErrOffline when none is
// connected. A send failure drops the device so the supervisor reopens it.
func (r *Reconnecting) Send(dst [4]byte, payload []byte) error {
	r.mu.Lock()
	d := r.cur
	r.mu.Unlock()
	r.txAll.Add(1)
	if d == nil {
		r.txErr.Add(1)
		return ErrOffline
	}
	if err := d.Send(dst, payload); err != nil {
		r.txErr.Add(1)
		r.drop(d, err)
		return err
	}
	return nil
}

// Receive polls the live device, or reports no packet while offline. A receive
// failure drops the device so the supervisor reopens it.
func (r *Reconnecting) Receive(buf []byte) (int, bool) {
	r.mu.Lock()
	d := r.cur
	r.mu.Unlock()
	if d == nil {
		return 0, false
	}
	var n int
	var ok bool
	if errorDongle, supportsErrors := d.(ReceiveErrorDongle); supportsErrors {
		var err error
		n, ok, err = errorDongle.ReceiveWithError(buf)
		if err != nil {
			r.drop(d, err)
			return 0, false
		}
	} else {
		n, ok = d.Receive(buf)
	}
	if ok {
		r.rxAll.Add(1)
	}
	return n, ok
}

// Close stops the supervisor and closes the live device, if any. After Close the
// dongle stays offline.
func (r *Reconnecting) Close() error {
	r.mu.Lock()
	r.closed = true
	d := r.cur
	r.cur = nil
	r.mu.Unlock()
	r.connected.Store(false)
	r.nudge()
	if d != nil {
		return d.Close()
	}
	return nil
}

// drop closes a failed device and clears it so the supervisor reopens it. It is
// a no-op if the current device has already been replaced by another goroutine.
func (r *Reconnecting) drop(d Dongle, err error) {
	r.mu.Lock()
	if r.cur != d {
		r.mu.Unlock()
		return
	}
	r.cur = nil
	r.mu.Unlock()
	r.connected.Store(false)
	r.lastDisconnect.Store(time.Now().Unix())
	d.Close()
	r.log.Warn("radio dongle disconnected; reconnecting", "err", err)
	r.nudge()
}

// nudge wakes the supervisor without blocking (the channel is buffered to 1, so
// a nudge that arrives while one is already pending is harmlessly coalesced).
func (r *Reconnecting) nudge() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

// supervise keeps a device connected: it reopens after a drop and, while offline,
// retries every backoff. It exits when ctx is cancelled or Close is called.
func (r *Reconnecting) supervise(ctx context.Context) {
	var loggedFail bool
	var opened bool // whether the device has ever connected (to count reopens)
	for {
		r.mu.Lock()
		closed := r.closed
		connected := r.cur != nil
		r.mu.Unlock()
		if closed {
			return
		}
		if connected {
			// Healthy: sleep until a drop nudges us (or shutdown).
			select {
			case <-ctx.Done():
				return
			case <-r.wake:
			}
			continue
		}

		d, err := r.open()
		if err != nil {
			if !loggedFail {
				// Log the first failure of an outage at warn (a typo'd selector or
				// a genuinely absent device), then stay quiet until it recovers.
				r.log.Warn("radio dongle unavailable; retrying", "err", err, "backoff", r.backoff)
				loggedFail = true
			}
			select {
			case <-ctx.Done():
				return
			case <-r.wake:
			case <-time.After(r.backoff):
			}
			continue
		}

		r.mu.Lock()
		if r.closed {
			r.mu.Unlock()
			d.Close()
			return
		}
		r.cur = d
		r.mu.Unlock()
		if opened {
			r.reconnects.Add(1)
		}
		opened = true
		r.lastConnect.Store(time.Now().Unix())
		r.connected.Store(true)
		r.log.Info("radio dongle connected")
		loggedFail = false
	}
}
