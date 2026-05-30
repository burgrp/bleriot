// This file adds Port, a self-healing radio that survives transport loss and
// reconnects automatically. See the modem package doc in modem.go.
package modem

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"hub/link"
)

// DefaultBackoff is the wait between (re)connection attempts when none is set.
const DefaultBackoff = 2 * time.Second

// ErrDisconnected is returned by Port.Send when no transport is currently
// connected.
var ErrDisconnected = errors.New("modem: radio disconnected")

// Opener opens the underlying byte stream for the radio (typically a serial
// port). It is called once per (re)connection attempt and must return a fresh
// stream each time.
type Opener func() (io.ReadWriteCloser, error)

// PortConfig configures a Port.
type PortConfig struct {
	// Open opens the transport. Required.
	Open Opener
	// Channel and Addr are applied to the radio (ConfigRadio) on every connect.
	Channel uint8
	Addr    [link.AddrLen]byte
	// RecvBuffer sets the capacity of the stable Received() channel.
	RecvBuffer int
	// Backoff is the wait between connection attempts (default DefaultBackoff).
	Backoff time.Duration
	// Logger logs connect/disconnect events and is passed to each underlying
	// modem. A nil logger disables logging.
	Logger *slog.Logger
}

// Port is a reconnecting radio. It owns one logical radio modem reached over a
// transport that may come and go (e.g. a USB serial device that is unplugged,
// disappears on a node reset, or is not present at startup). It implements the
// engine's Radio interface with a Received() channel that is stable across
// reconnects, so the engine can register it once and keep using it. Create with
// NewPort and call Run in its own goroutine. Send and Received are safe for
// concurrent use.
type Port struct {
	open    Opener
	channel uint8
	addr    [link.AddrLen]byte
	backoff time.Duration
	log     *slog.Logger

	recv chan [PacketLen]byte

	mu      sync.Mutex
	current *Modem
}

// NewPort creates a reconnecting Port from cfg.
func NewPort(cfg PortConfig) *Port {
	buf := cfg.RecvBuffer
	if buf < 1 {
		buf = 16
	}
	bo := cfg.Backoff
	if bo <= 0 {
		bo = DefaultBackoff
	}
	log := cfg.Logger
	if log == nil {
		log = slog.New(discardHandler{})
	}
	return &Port{
		open:    cfg.Open,
		channel: cfg.Channel,
		addr:    cfg.Addr,
		backoff: bo,
		log:     log,
		recv:    make(chan [PacketLen]byte, buf),
	}
}

// Received delivers BleRiot packets from whichever modem is currently connected.
// The channel is stable across reconnects and is closed only when Run returns
// (ctx cancellation).
func (p *Port) Received() <-chan [PacketLen]byte { return p.recv }

// Send transmits one BleRiot packet via the currently connected modem. It
// returns ErrDisconnected if no transport is connected right now; the caller's
// retry/timeout logic (the engine) treats this like any other send failure.
func (p *Port) Send(dst [link.AddrLen]byte, payload []byte) error {
	p.mu.Lock()
	m := p.current
	p.mu.Unlock()
	if m == nil {
		return ErrDisconnected
	}
	return m.Send(dst, payload)
}

// Run drives the connect/serve/reconnect loop until ctx is cancelled. It always
// closes the Received() channel before returning.
func (p *Port) Run(ctx context.Context) {
	defer close(p.recv)
	for {
		if ctx.Err() != nil {
			return
		}
		if !p.connectAndServe(ctx) {
			return
		}
		if !sleepCtx(ctx, p.backoff) {
			return
		}
	}
}

// connectAndServe makes one connection attempt and serves it until it fails or
// ctx is cancelled. It returns false only when ctx is cancelled (Run should
// stop); true means the caller should back off and retry.
func (p *Port) connectAndServe(ctx context.Context) bool {
	transport, err := p.open()
	if err != nil {
		p.log.Warn("radio open failed; will retry", "backoff", p.backoff, "err", err)
		return ctx.Err() == nil
	}
	p.log.Info("radio connected")

	m := New(transport, cap(p.recv), WithLogger(p.log))
	p.setCurrent(m)
	defer func() {
		p.setCurrent(nil)
		_ = transport.Close() // release the fd before reconnecting
	}()

	// Pump received packets onto the stable channel until the modem stops
	// (m.Run closes m.Received() on exit, ending the range).
	pumpDone := make(chan struct{})
	go func() {
		defer close(pumpDone)
		for pkt := range m.Received() {
			select {
			case p.recv <- pkt:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Apply the radio configuration for this connection.
	if err := m.ConfigRadio(p.channel, p.addr); err != nil {
		p.log.Warn("radio configure failed", "err", err)
	}

	err = m.Run(ctx)
	<-pumpDone

	if ctx.Err() != nil {
		return false
	}
	p.log.Warn("radio disconnected; reconnecting", "backoff", p.backoff, "err", err)
	return true
}

func (p *Port) setCurrent(m *Modem) {
	p.mu.Lock()
	p.current = m
	p.mu.Unlock()
}

// sleepCtx waits d or until ctx is cancelled. It returns false if ctx was
// cancelled (caller should stop).
func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}
