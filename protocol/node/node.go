// Package node is the firmware-side BleRiot runtime: it owns the radio receive
// loop, the XTEA codec, and the GET/SET/WATCH dispatch defined in PROTOCOL.md
// (§7–§8), so a device firmware only has to implement two switches — one to read
// a register by tag and one to write a register by tag.
//
// It has no external dependencies and no build tags, so it unit-tests on the
// host and compiles under TinyGo alike. After construction the steady-state loop
// never allocates: all buffers live on the Node.
package node

import "protocol"

// Radio is the minimal transport the runtime needs. It matches the way the modem
// firmware drives its PAN211x (see hub/fw): the caller configures the channel and
// receive address once, before handing the radio to the Node.
type Radio interface {
	// Send transmits a single packet to dst. It must not retain packet.
	Send(dst [4]byte, packet []byte) error
	// Receive copies at most one received packet into buf and reports how many
	// bytes were written and whether a packet was available. It must not block.
	Receive(buf []byte) (n int, ok bool)
}

// Device is the application behind the registers. The runtime calls Read for a
// GET and Write for a SET, both identified by the register's permanent tag
// (PROTOCOL.md §11). A typical implementation is a switch over the device's tag
// constants.
//
// Both Read and Notify carry the register's value together with a null flag:
// when null is true the register currently has no value (sensor not ready,
// register unset, unknown tag, …) and the runtime replies with the NULL flag set
// and value 0.
type Device interface {
	// Read returns the current value of the register identified by tag.
	Read(tag uint16) (value int32, null bool)
	// Write applies value to the register identified by tag. null reports the
	// hub's NULL flag: when true the hub is clearing the register (value is
	// undefined and must be ignored). Write has no return value: the write may be
	// processed asynchronously and take time. The runtime acknowledges a SET with
	// an ACK packet (PROTOCOL.md §8.2) carrying no value, so Write need not
	// produce one; the settled value reaches a watching hub later via Notify.
	// Writes to unknown or read-only registers are ignored.
	Write(tag uint16, value int32, null bool)
}

// maxSubs bounds the watch table. Each entry is one (hub address, tag) pair; the
// table is a fixed array so the runtime never allocates. When it is full a new
// subscription evicts the oldest entry.
const maxSubs = 16

type sub struct {
	addr   [4]byte
	tag    uint16
	active bool
}

// Node is the firmware runtime for a single device. Create one with New, then
// call Run (or drive Poll yourself). All fields are owned by the Node; the
// steady-state loop is allocation-free.
type Node struct {
	radio Radio
	codec protocol.Codec
	dev   Device
	self  [4]byte

	subs    [maxSubs]sub
	nextSub int

	rxBuf [protocol.PacketLen]byte
	txBuf [protocol.PacketLen]byte
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
// packet. Poll is exported so firmware can interleave other work (and so tests
// can step the runtime deterministically).
func (n *Node) Poll() bool {
	m, ok := n.radio.Receive(n.rxBuf[:])
	if !ok || m != protocol.PacketLen {
		return false
	}
	src, typ, flags, reg, value, err := n.codec.Decode(n.rxBuf[:])
	if err != nil {
		return false
	}
	switch typ {
	case protocol.TypeGET:
		v, null := n.dev.Read(reg)
		n.replyIS(src, reg, v, null)
	case protocol.TypeSET:
		n.dev.Write(reg, value, flags&protocol.FlagNULL != 0)
		// Acknowledge receipt with an ACK (PROTOCOL.md §8.2); it carries no value.
		// The write may settle asynchronously — the eventual value is pushed to
		// any watcher via Notify.
		n.send(src, protocol.TypeACK, 0, reg, 0)
	case protocol.TypeWATCH:
		if value != 0 {
			n.subscribe(src, reg)
			v, null := n.dev.Read(reg)
			n.replyIS(src, reg, v, null)
		} else {
			n.unsubscribe(src, reg)
		}
	default:
		// IS is node→hub only; a node never receives one. Ignore anything else.
	}
	return true
}

// Notify pushes the new value of a register to every hub currently watching it.
// Firmware calls this whenever a register's value changes on its own (a sensor
// reading, a relay toggled locally, …) so subscribers are kept up to date
// (PROTOCOL.md §8, WATCH). When null is true the register has become unset and
// the push carries the NULL flag with value 0 (the dual of a NULL IS reply).
func (n *Node) Notify(tag uint16, value int32, null bool) {
	for i := range n.subs {
		s := &n.subs[i]
		if s.active && s.tag == tag {
			n.replyIS(s.addr, tag, value, null)
		}
	}
}

// replyIS sends an IS packet back to dst with the NULL flag set when null.
func (n *Node) replyIS(dst [4]byte, reg uint16, value int32, null bool) {
	var flags byte
	if null {
		flags = protocol.FlagNULL
		value = 0
	}
	n.send(dst, protocol.TypeIS, flags, reg, value)
}

// send encodes one packet into txBuf and hands it to the radio.
func (n *Node) send(dst [4]byte, typ, flags byte, reg uint16, value int32) {
	n.codec.Encode(n.txBuf[:], n.self, typ, flags, reg, value)
	_ = n.radio.Send(dst, n.txBuf[:])
}

// subscribe records a (hub, tag) watch, refreshing it if it already exists.
// A full table evicts the oldest entry (round-robin via nextSub).
func (n *Node) subscribe(addr [4]byte, tag uint16) {
	var free *sub
	for i := range n.subs {
		s := &n.subs[i]
		if s.active && s.addr == addr && s.tag == tag {
			return // already subscribed
		}
		if free == nil && !s.active {
			free = s
		}
	}
	if free == nil {
		free = &n.subs[n.nextSub]
		n.nextSub = (n.nextSub + 1) % maxSubs
	}
	free.addr = addr
	free.tag = tag
	free.active = true
}

// unsubscribe drops a (hub, tag) watch if present.
func (n *Node) unsubscribe(addr [4]byte, tag uint16) {
	for i := range n.subs {
		s := &n.subs[i]
		if s.active && s.addr == addr && s.tag == tag {
			s.active = false
			return
		}
	}
}
