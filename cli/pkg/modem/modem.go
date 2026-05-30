// Package modem is the host-side client for a single BleRiot "dumb radio modem"
// reached over one serial port. It wraps the COBS-framed link protocol (the
// hub/link package) and exposes a small, transport-agnostic API: configure
// the radio, send BleRiot packets, and receive packets the radio picked up.
//
// One Modem == one radio == one serial port. Routing across several modems
// (one per channel/port) lives above this package.
package modem

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"

	"hub/link"
)

// PacketLen is the fixed BleRiot on-wire packet size (PROTOCOL.md §4).
const PacketLen = 13

// maxFrame bounds a single decoded link frame: a MsgSend/MsgRecv body is at most
// a type byte + address + one packet, well under this.
const maxFrame = 64

// ErrClosed is returned by Send/ConfigRadio after the modem's run loop has
// stopped (port closed or context cancelled).
var ErrClosed = errors.New("modem: closed")

// Modem is the host endpoint for one radio modem. Create with New, then call Run
// in its own goroutine to service the serial port. Send and ConfigRadio are safe
// for concurrent use.
type Modem struct {
	port io.ReadWriteCloser
	log  *slog.Logger

	wmu sync.Mutex // serialises writes to port
	buf []byte     // reusable encode buffer, guarded by wmu

	recv chan [PacketLen]byte
	errs chan byte

	mu      sync.Mutex
	version uint8
	closed  bool
}

// Option configures a Modem in New.
type Option func(*Modem)

// WithLogger attaches an slog.Logger. The modem logs at debug level: each serial
// frame written ("modem tx") and read chunk ("modem rx"), plus decoded inbound
// messages. Pass a logger whose handler is enabled for slog.LevelDebug to see
// the serial communication. A nil logger is ignored.
func WithLogger(l *slog.Logger) Option {
	return func(m *Modem) {
		if l != nil {
			m.log = l
		}
	}
}

// New creates a Modem over the given byte stream (a serial port, or any
// io.ReadWriteCloser for testing). recvBuffer sets the capacity of the received
// packet channel.
func New(port io.ReadWriteCloser, recvBuffer int, opts ...Option) *Modem {
	if recvBuffer < 1 {
		recvBuffer = 16
	}
	m := &Modem{
		port: port,
		log:  slog.New(discardHandler{}),
		buf:  make([]byte, 0, maxFrame),
		recv: make(chan [PacketLen]byte, recvBuffer),
		errs: make(chan byte, 8),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// discardHandler is a no-op slog.Handler used when no logger is supplied, so the
// modem can always call m.log without nil checks.
type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (discardHandler) Handle(context.Context, slog.Record) error { return nil }
func (h discardHandler) WithAttrs([]slog.Attr) slog.Handler      { return h }
func (h discardHandler) WithGroup(string) slog.Handler           { return h }

// Received delivers BleRiot packets reported by the radio (one MsgRecv each).
// Closed when Run returns.
func (m *Modem) Received() <-chan [PacketLen]byte { return m.recv }

// Errors delivers modem-side error codes (link.Err*). Best-effort: codes are
// dropped rather than blocking if the consumer is slow. Closed when Run returns.
func (m *Modem) Errors() <-chan byte { return m.errs }

// ProtocolVersion returns the link protocol version reported by the modem's
// MsgHello, or 0 if no hello has been seen yet.
func (m *Modem) ProtocolVersion() uint8 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.version
}

// ConfigRadio sets the radio's channel and receive address (PROTOCOL.md §2).
func (m *Modem) ConfigRadio(channel uint8, addr [link.AddrLen]byte) error {
	return m.write(link.Message{Type: link.MsgConfigRadio, Channel: channel, Addr: addr})
}

// Send transmits one BleRiot packet to dst. payload must be PacketLen bytes.
func (m *Modem) Send(dst [link.AddrLen]byte, payload []byte) error {
	if len(payload) != PacketLen {
		return errors.New("modem: payload must be exactly PacketLen bytes")
	}
	return m.write(link.Message{Type: link.MsgSend, Addr: dst, Payload: payload})
}

func (m *Modem) write(msg link.Message) error {
	m.mu.Lock()
	closed := m.closed
	m.mu.Unlock()
	if closed {
		return ErrClosed
	}

	m.wmu.Lock()
	defer m.wmu.Unlock()
	m.buf = m.buf[:0]
	m.buf = link.AppendMessage(m.buf, msg)
	if m.log.Enabled(context.Background(), slog.LevelDebug) {
		m.log.Debug("modem tx",
			"type", msgTypeName(msg.Type),
			"bytes", len(m.buf),
			"frame", hex(m.buf))
	}
	_, err := m.port.Write(m.buf)
	if err != nil {
		m.log.Debug("modem tx error", "err", err)
	}
	return err
}

// Run services the serial port until ctx is cancelled or a read error occurs
// (e.g. the port closes). It always closes the Received and Errors channels and
// marks the modem closed before returning. The returned error is the cause of
// the read loop ending (nil on clean ctx cancellation).
func (m *Modem) Run(ctx context.Context) error {
	defer func() {
		m.mu.Lock()
		m.closed = true
		m.mu.Unlock()
		close(m.recv)
		close(m.errs)
	}()

	// Unblock the blocking Read on ctx cancellation by closing the port.
	stop := context.AfterFunc(ctx, func() { _ = m.port.Close() })
	defer stop()

	dec := link.NewDecoder(maxFrame)
	rd := make([]byte, 256)
	for {
		n, err := m.port.Read(rd)
		if n > 0 && m.log.Enabled(ctx, slog.LevelDebug) {
			m.log.Debug("modem rx", "bytes", n, "data", hex(rd[:n]))
		}
		for i := 0; i < n; i++ {
			msg, ok, derr := dec.Push(rd[i])
			if derr != nil {
				// Framing error: resynchronises automatically; surface as error.
				m.report(link.ErrBadFrame)
				continue
			}
			if ok {
				m.dispatch(msg)
			}
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (m *Modem) dispatch(msg link.Message) {
	switch msg.Type {
	case link.MsgRecv:
		if len(msg.Payload) != PacketLen {
			m.report(link.ErrBadFrame)
			return
		}
		var pkt [PacketLen]byte
		copy(pkt[:], msg.Payload)
		m.log.Debug("modem recv packet", "payload", hex(pkt[:]))
		select {
		case m.recv <- pkt:
		default: // drop if the consumer is not keeping up
			m.log.Debug("modem recv dropped: consumer not keeping up")
		}
	case link.MsgHello:
		m.mu.Lock()
		m.version = msg.Version
		m.mu.Unlock()
		m.log.Debug("modem hello", "version", msg.Version)
	case link.MsgError:
		m.log.Debug("modem error message", "code", msg.Code)
		m.report(msg.Code)
	}
}

func (m *Modem) report(code byte) {
	select {
	case m.errs <- code:
	default:
	}
}

// hex renders bytes as an uppercase, space-separated hex string for logging.
func hex(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	const digits = "0123456789ABCDEF"
	out := make([]byte, 0, len(b)*3-1)
	for i, c := range b {
		if i > 0 {
			out = append(out, ' ')
		}
		out = append(out, digits[c>>4], digits[c&0x0F])
	}
	return string(out)
}

func msgTypeName(t link.MsgType) string {
	switch t {
	case link.MsgHello:
		return "Hello"
	case link.MsgConfigRadio:
		return "ConfigRadio"
	case link.MsgSend:
		return "Send"
	case link.MsgRecv:
		return "Recv"
	case link.MsgError:
		return "Error"
	default:
		return "Unknown"
	}
}
