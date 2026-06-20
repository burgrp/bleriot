package radio

import (
	"context"
	"errors"
	"log/slog"
	"sync"
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
// The reply guard (PROTOCOL.md §6) is fixed at construction: it depends only on
// the channel's spreading factor, not the hardware, so the engine can validate
// and use it before any device is present. This lets the hub start with every
// dongle disconnected.
//
// A disconnect is detected from a Send failure: the hub is the master and sends
// at least once per watch-refresh interval, so a vanished device surfaces
// quickly. The failed device is closed and the supervisor reopens it; Receive
// simply yields no packets while offline. A selector that re-resolves the device
// each open (e.g. a USB serial rather than a fixed /dev/hidraw path) reconnects
// even when the device returns on a different node.
type Reconnecting struct {
	open    func() (Dongle, error)
	guard   time.Duration
	backoff time.Duration
	log     *slog.Logger

	wake chan struct{} // buffered(1): nudges the supervisor to reopen after a drop

	mu     sync.Mutex
	cur    Dongle // the live device, or nil while offline
	closed bool
}

// NewReconnecting creates a Reconnecting dongle and starts its supervisor, which
// keeps a device connected until ctx is cancelled or Close is called. open is
// called to (re)open the physical device; guard is the device's reply turnaround
// (PROTOCOL.md §6); backoff is the wait between reconnection attempts (defaults
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

// ReplyGuard reports the reply turnaround guard (PROTOCOL.md §6), known up front
// from the channel's spreading factor and independent of the live device.
func (r *Reconnecting) ReplyGuard() time.Duration { return r.guard }

// Send transmits via the live device, or reports ErrOffline when none is
// connected. A send failure drops the device so the supervisor reopens it.
func (r *Reconnecting) Send(dst [4]byte, payload []byte) error {
	r.mu.Lock()
	d := r.cur
	r.mu.Unlock()
	if d == nil {
		return ErrOffline
	}
	if err := d.Send(dst, payload); err != nil {
		r.drop(d, err)
		return err
	}
	return nil
}

// Receive polls the live device, or reports no packet while offline.
func (r *Reconnecting) Receive(buf []byte) (int, bool) {
	r.mu.Lock()
	d := r.cur
	r.mu.Unlock()
	if d == nil {
		return 0, false
	}
	return d.Receive(buf)
}

// Close stops the supervisor and closes the live device, if any. After Close the
// dongle stays offline.
func (r *Reconnecting) Close() error {
	r.mu.Lock()
	r.closed = true
	d := r.cur
	r.cur = nil
	r.mu.Unlock()
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
		r.log.Info("radio dongle connected")
		loggedFail = false
	}
}
