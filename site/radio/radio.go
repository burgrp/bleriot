// Package radio adapts an RF dongle to the BleRiot hub and node radio
// interfaces. It is transport-agnostic: a Dongle is any single-channel RF
// endpoint that can Send and Receive whole packets. The MCP2210 + PAN211x
// USB-to-SPI bridge (subpackage mcpdongle) is one implementation; a smart
// MCU-resident dongle speaking a framed USB protocol would be another, slotting
// in with no change to the engine, the node runtime, or this package.
package radio

import (
	"context"
	"time"

	"protocol"
)

// pollInterval is how often the hub receive loop checks the dongle for an
// inbound packet. The hub is the master in every transaction, so replies arrive
// well within the engine's per-attempt timeout; a short interval keeps latency
// low for dongles without a packet-ready push (e.g. the MCP2210 has no interrupt
// line wired to the host).
const pollInterval = time.Millisecond

// Dongle is one RF endpoint: a radio brought up on a single channel and receive
// address, reachable over some host transport. Implementations serialise their
// own access as needed. Receive is non-blocking; Close releases the dongle and
// any underlying device.
type Dongle interface {
	// Send transmits payload to dst. It blocks until transmission completes.
	Send(dst [4]byte, payload []byte) error
	// Receive copies at most one received packet into buf and reports how many
	// bytes were written and whether a packet was available. It never blocks.
	Receive(buf []byte) (n int, ok bool)
	// ReplyGuard reports the reply turnaround guard (protocol/README.md §6) the hub must
	// ask nodes to wait before answering a request sent through this dongle, so a
	// slow half-duplex dongle has switched back to receive in time. It is a
	// per-dongle constant.
	ReplyGuard() time.Duration
	// Close releases the dongle (and any underlying device).
	Close() error
}

// Radio adapts a Dongle to the hub engine's Radio interface (Send / Received):
// it runs a receive loop that polls the dongle and forwards each complete packet
// on a channel.
type Radio struct {
	d    Dongle
	in   chan [protocol.PacketLen]byte
	done chan struct{}
}

// New starts a receive loop over d that runs until ctx is cancelled, at which
// point it closes the dongle and the Received channel. The returned Radio
// satisfies the engine's Radio interface.
func New(ctx context.Context, d Dongle) *Radio {
	r := &Radio{
		d:    d,
		in:   make(chan [protocol.PacketLen]byte, 16),
		done: make(chan struct{}),
	}
	go r.recvLoop(ctx)
	return r
}

// Send transmits payload to dst.
func (r *Radio) Send(dst [4]byte, payload []byte) error {
	return r.d.Send(dst, payload)
}

// ReplyGuard reports the dongle's reply turnaround guard (protocol/README.md §6),
// forwarded to the engine so it can ask nodes to defer their replies accordingly.
func (r *Radio) ReplyGuard() time.Duration { return r.d.ReplyGuard() }

// Received returns the channel of inbound packets. It is closed once ctx (passed
// to New) is cancelled.
func (r *Radio) Received() <-chan [protocol.PacketLen]byte {
	return r.in
}

// Done returns a channel that is closed once the receive loop has exited and the
// dongle has been closed (after the ctx passed to New is cancelled). Callers that
// reuse the underlying device should wait on it before re-opening, to avoid
// overlapping sessions on the same hardware.
func (r *Radio) Done() <-chan struct{} {
	return r.done
}

// recvLoop polls the dongle for received packets and forwards complete ones,
// closing the dongle and the inbound channel when ctx is cancelled.
func (r *Radio) recvLoop(ctx context.Context) {
	defer close(r.done)
	defer close(r.in)
	defer r.d.Close()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	buf := make([]byte, protocol.PacketLen)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, ok := r.d.Receive(buf)
			if !ok || n != protocol.PacketLen {
				continue
			}
			var pkt [protocol.PacketLen]byte
			copy(pkt[:], buf)
			select {
			case r.in <- pkt:
			case <-ctx.Done():
				return
			}
		}
	}
}
