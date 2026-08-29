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
// is connected. The engine treats it like any other send failure, and the
// bridge's continuing polling sweeps report a node on that channel unavailable
// until the device returns.
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

	// Diagnostic counters are updated lock-free; connected mirrors cur != nil.
	connected               atomic.Bool
	lastConnect             atomic.Int64
	lastDisconnect          atomic.Int64
	txAll                   atomic.Uint64 // Send calls
	txErr                   atomic.Uint64 // Send calls that failed (including while offline)
	rxAll                   atomic.Uint64 // packets actually received
	stateSince              atomic.Int64
	openAttempts            atomic.Uint64
	openSuccesses           atomic.Uint64
	openFailures            atomic.Uint64
	lastOpenAttempt         atomic.Int64
	lastOpenFailure         atomic.Int64
	disconnects             atomic.Uint64
	disconnectSendErrors    atomic.Uint64
	disconnectReceiveErrors atomic.Uint64
	txSuccess               atomic.Uint64
	txOffline               atomic.Uint64
	rxErrors                atomic.Uint64
}

// DongleState is the lifecycle state of a reconnecting channel endpoint.
type DongleState uint32

const (
	DongleOffline DongleState = iota
	DongleConnected
	DongleClosed
)

// DongleStats is a point-in-time snapshot of a Reconnecting dongle's connection
// state and cumulative traffic counters, as published by the diagnostics bridge.
type DongleStats struct {
	State                   DongleState
	StateSince              int64
	OpenAttempts            uint64
	OpenSuccesses           uint64
	OpenFailures            uint64
	LastOpenAttempt         int64
	LastOpenFailure         int64
	Disconnects             uint64
	DisconnectSendErrors    uint64
	DisconnectReceiveErrors uint64
	LastConnected           int64
	LastDisconnected        int64
	TxAttempts              uint64
	TxSuccess               uint64
	TxOffline               uint64
	TxErrors                uint64
	RxSuccess               uint64
	RxErrors                uint64
}

// Stats returns a snapshot of the dongle's connection state and traffic counters.
func (r *Reconnecting) Stats() DongleStats {
	state := DongleOffline
	if r.closedState() {
		state = DongleClosed
	} else if r.connected.Load() {
		state = DongleConnected
	}
	offline := r.txOffline.Load()
	txErrors := r.txErr.Load() - offline
	return DongleStats{
		State:                   state,
		StateSince:              r.stateSince.Load(),
		OpenAttempts:            r.openAttempts.Load(),
		OpenSuccesses:           r.openSuccesses.Load(),
		OpenFailures:            r.openFailures.Load(),
		LastOpenAttempt:         r.lastOpenAttempt.Load(),
		LastOpenFailure:         r.lastOpenFailure.Load(),
		Disconnects:             r.disconnects.Load(),
		DisconnectSendErrors:    r.disconnectSendErrors.Load(),
		DisconnectReceiveErrors: r.disconnectReceiveErrors.Load(),
		LastConnected:           r.lastConnect.Load(),
		LastDisconnected:        r.lastDisconnect.Load(),
		TxAttempts:              r.txAll.Load(),
		TxSuccess:               r.txSuccess.Load(),
		TxOffline:               offline,
		TxErrors:                txErrors,
		RxSuccess:               r.rxAll.Load(),
		RxErrors:                r.rxErrors.Load(),
	}
}

func (r *Reconnecting) closedState() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
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
	r.stateSince.Store(time.Now().Unix())
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
		r.txOffline.Add(1)
		return ErrOffline
	}
	if err := d.Send(dst, payload); err != nil {
		r.txErr.Add(1)
		r.drop(d, disconnectSend, err)
		return err
	}
	r.txSuccess.Add(1)
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
			r.rxErrors.Add(1)
			r.drop(d, disconnectReceive, err)
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
	r.stateSince.Store(time.Now().Unix())
	r.nudge()
	if d != nil {
		return d.Close()
	}
	return nil
}

// drop closes a failed device and clears it so the supervisor reopens it. It is
// a no-op if the current device has already been replaced by another goroutine.
type disconnectCause uint8

const (
	disconnectSend disconnectCause = iota
	disconnectReceive
)

func (r *Reconnecting) drop(d Dongle, cause disconnectCause, err error) {
	r.mu.Lock()
	if r.cur != d {
		r.mu.Unlock()
		return
	}
	r.cur = nil
	r.mu.Unlock()
	r.connected.Store(false)
	now := time.Now().Unix()
	r.lastDisconnect.Store(now)
	r.stateSince.Store(now)
	r.disconnects.Add(1)
	if cause == disconnectSend {
		r.disconnectSendErrors.Add(1)
	} else {
		r.disconnectReceiveErrors.Add(1)
	}
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

		now := time.Now().Unix()
		r.openAttempts.Add(1)
		r.lastOpenAttempt.Store(now)
		d, err := r.open()
		if err != nil {
			r.openFailures.Add(1)
			r.lastOpenFailure.Store(time.Now().Unix())
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
		now = time.Now().Unix()
		r.openSuccesses.Add(1)
		r.lastConnect.Store(now)
		r.stateSince.Store(now)
		r.connected.Store(true)
		r.log.Info("radio dongle connected")
		loggedFail = false
	}
}
