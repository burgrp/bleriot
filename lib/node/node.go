// Package node is the firmware-side BleRiot runtime: it owns the radio receive
// loop, the XTEA codec, and the serialized GET/SET dispatch, so device firmware
// only has to implement reading and writing registers by permanent tag.
//
// It has no external dependencies and no build tags, so it unit-tests on the
// host and compiles under TinyGo alike. After construction the steady-state loop
// never allocates: all buffers live on the Node.
package node

import (
	_ "unsafe" // for go:linkname of runtime.nanotime

	"time"

	"github.com/burgrp/bleriot/lib/shared/protocol"
)

const (
	onlineTimeout   = 5 * time.Second
	statusLEDPeriod = time.Second
	statusLEDOn     = 200 * time.Millisecond
)

// nanotime returns a monotonic clock reading in nanoseconds. The epoch is
// unspecified; status tracking only compares readings and deadlines.
//
//go:linkname nanotime runtime.nanotime
func nanotime() int64

// Radio is the minimal transport the runtime needs. It matches the way a PAN211x
// is driven directly: the caller configures the channel and receive address
// once, before handing the radio to the Node.
type Radio interface {
	// Send transmits a single packet to dst. It must not retain packet.
	Send(dst [4]byte, packet []byte) error
	// Receive copies at most one received packet into buf and reports how many
	// bytes were written and whether a packet was available. It must not block.
	Receive(buf []byte) (n int, ok bool)
}

// Device is the application behind the registers. A typical implementation is a
// switch over the device's permanent register tags.
type Device interface {
	// Read returns the current value for a GET. When null is true the register
	// currently has no value and the runtime sends VALUE with NULL and value 0.
	Read(tag uint16) (value int32, null bool)
	// Write accepts or starts an idempotent absolute assignment for SET. The
	// physical action may settle asynchronously after Write returns. When null is
	// true the hub is clearing the register and value is undefined. Duplicate SET
	// retries may call Write again and must be harmless. Unknown or read-only tags
	// are ignored.
	Write(tag uint16, value int32, null bool)
}

// Node is the firmware runtime for a single device. Create one with New, then
// call Run (or drive Poll yourself). All fields are owned by the Node; the
// steady-state loop is allocation-free.
type Node struct {
	radio Radio
	codec protocol.Codec
	dev   Device
	self  [4]byte

	rxBuf [protocol.PacketLen]byte
	txBuf [protocol.PacketLen]byte

	statusStart       time.Duration
	onlineUntil       time.Duration
	statusInitialized bool
	statusOnline      bool
	statusLED         bool
}

// New builds a Node for a device whose RF source address is self and whose shared
// XTEA key is key. radio must already be configured for the device's channel and
// receive address.
func New(radio Radio, self [4]byte, key [16]byte, dev Device) (*Node, error) {
	codec, err := protocol.NewCodec(key)
	if err != nil {
		return nil, err
	}
	return &Node{radio: radio, codec: codec, dev: dev, self: self}, nil
}

// Run drives the runtime forever: it polls the radio and dispatches each received
// request. It never returns. Firmware typically calls this at the end of Start.
func (n *Node) Run() {
	for {
		n.Poll()
	}
}

// Poll performs one iteration of the runtime: if a packet is waiting it is
// decoded and dispatched. It is non-blocking and returns true when it handled a
// packet. Every handled packet refreshes the liveness state reported by
// PollWithStatus. Poll is exported so firmware can interleave other work (and
// so tests can step the runtime deterministically).
func (n *Node) Poll() bool {
	m, ok := n.radio.Receive(n.rxBuf[:])
	if !ok || m != protocol.PacketLen {
		return false
	}
	src, typ, flags, reg, value, err := n.codec.Decode(n.rxBuf[:])
	if err != nil {
		return false
	}
	n.receivedAt(statusNow())
	switch typ {
	case protocol.TypeGET:
		current, null := n.dev.Read(reg)
		n.waitGuard(flags)
		n.replyValue(src, flags, reg, current, null)
	case protocol.TypeSET:
		n.waitGuard(flags)
		// ACK confirms receipt before device handling, which may start an
		// asynchronous physical action. A later GET observes the resulting state.
		ackFlags := protocol.GuardFlags(flags) | flags&protocol.FlagNULL
		n.send(src, protocol.TypeACK, ackFlags, reg, 0)
		n.dev.Write(reg, value, flags&protocol.FlagNULL != 0)
	default:
		// VALUE and ACK are node→hub only. Consuming one produces no response.
	}
	return true
}

// PollWithStatus performs one Poll and returns the node's connection and status
// LED state. online remains true for five seconds after the most recent packet
// handled by Poll. led follows a one-second heartbeat that is true for 200 ms
// and false for 800 ms, independently of online. changed is true on the first
// call and whenever online or led differs from the preceding call.
//
// Callers should invoke PollWithStatus frequently enough to observe the LED
// edges they need to drive.
func (n *Node) PollWithStatus() (online, led, changed bool) {
	n.Poll()
	return n.statusAt(statusNow())
}

func statusNow() time.Duration {
	return time.Duration(nanotime())
}

func (n *Node) receivedAt(now time.Duration) {
	n.onlineUntil = now + onlineTimeout
}

func (n *Node) statusAt(now time.Duration) (online, led, changed bool) {
	first := !n.statusInitialized
	if first {
		n.statusStart = now
	}

	online = now < n.onlineUntil
	led = (now-n.statusStart)%statusLEDPeriod < statusLEDOn
	changed = first || online != n.statusOnline || led != n.statusLED

	n.statusInitialized = true
	n.statusOnline = online
	n.statusLED = led
	return
}

// waitGuard gives the hub's half-duplex radio time to switch from transmit to
// receive. A response echoes the field as metadata but never interprets it as a
// second wait.
func (n *Node) waitGuard(flags byte) {
	if guard := protocol.GuardMillis(flags); guard != 0 {
		time.Sleep(time.Duration(guard) * time.Millisecond)
	}
}

// replyValue sends the direct result of a GET. GUARD comes from the request;
// NULL is determined only by Device.Read, not by the request's NULL bit.
func (n *Node) replyValue(dst [4]byte, requestFlags byte, reg uint16, value int32, null bool) {
	flags := protocol.GuardFlags(requestFlags)
	if null {
		flags |= protocol.FlagNULL
		value = 0
	}
	n.send(dst, protocol.TypeVALUE, flags, reg, value)
}

// send encodes the transaction's single response into txBuf and hands it to the
// radio. There are no unsolicited packets or response cache.
func (n *Node) send(dst [4]byte, typ, flags byte, reg uint16, value int32) {
	n.codec.Encode(n.txBuf[:], n.self, typ, flags, reg, value)
	_ = n.radio.Send(dst, n.txBuf[:])
}
