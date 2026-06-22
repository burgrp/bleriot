// Package node is the firmware-side BleRiot runtime: it owns the radio receive
// loop, the XTEA codec, and the GET/SET/WATCH dispatch defined in protocol/README.md
// (§7–§8), so a device firmware only has to implement two switches — one to read
// a register by tag and one to write a register by tag.
//
// It has no external dependencies and no build tags, so it unit-tests on the
// host and compiles under TinyGo alike. After construction the steady-state loop
// never allocates: all buffers live on the Node.
package node

import (
	"time"

	"protocol"
)

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

// Device is the application behind the registers. The runtime calls Read for a
// GET and Write for a SET, both identified by the register's permanent tag
// (protocol/README.md §11). A typical implementation is a switch over the device's tag
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
	// an ACK packet (protocol/README.md §8.2) carrying no value, so Write need not
	// produce one; the settled value reaches a watching hub later via Notify.
	// Writes to unknown or read-only registers are ignored.
	Write(tag uint16, value int32, null bool)
}

// maxSubs bounds the watch table. Each entry is one (hub address, tag) pair; the
// table is a fixed array so the runtime never allocates. When it is full a new
// subscription evicts the oldest entry.
const maxSubs = 16

// Spontaneous-push reliability (protocol/README.md §8.3, §9). A push (from Notify) has
// no outstanding hub request behind it, so unlike a solicited reply it cannot be
// recovered by the hub retransmitting — the node retransmits the push itself
// until the hub ACKs it. pushRetryInterval is how long the node waits for that
// ACK before resending (pushRetryMillis is the same value as a nowMillis tick),
// and pushMaxTries bounds the attempts so a vanished hub does not draw the radio
// forever (after which the push is abandoned, as the next WATCH refresh will
// re-read the register anyway). maxPending bounds the in-flight push table; a
// further push beyond it evicts the oldest (see pendingSlot).
const (
	pushRetryInterval = 60 * time.Millisecond
	pushRetryMillis   = uint32(pushRetryInterval / time.Millisecond)
	pushMaxTries      = 16
	maxPending        = 8
)

type sub struct {
	addr   [4]byte
	tag    uint16
	active bool
}

// pendingPush is one in-flight spontaneous push awaiting the hub's ACK. The
// table is a fixed array (maxPending entries) so Notify and the retransmit pump
// never allocate. A newer push for the same (addr, tag) supersedes the older
// value in place. deadline is the next-retransmit time as a millisecond tick
// (nowMillis), kept as a uint32 rather than a time.Time to shrink the table on a
// memory-constrained node; comparisons are wrap-safe (see servicePending).
type pendingPush struct {
	addr     [4]byte
	value    int32
	deadline uint32
	tries    int
	tag      uint16
	null     bool
	active   bool
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

	// pending holds spontaneous pushes awaiting the hub's ACK; nextPending is the
	// round-robin eviction cursor when the table is full.
	pending     [maxPending]pendingPush
	nextPending int

	// guard is the hub's turnaround guard (protocol/README.md §6) learned from the GUARD
	// field of the last request. lastTx is when the last packet was handed to the
	// radio. Together they pace consecutive transmits so the hub's half-duplex
	// dongle has time to read one packet out and re-arm before the next arrives.
	guard  time.Duration
	lastTx time.Time

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
		n.servicePending()
		return false
	}
	src, typ, flags, reg, value, err := n.codec.Decode(n.rxBuf[:])
	if err != nil {
		n.servicePending()
		return false
	}
	// Remember the hub's turnaround guard (protocol/README.md §6) from this request's
	// GUARD field. It is used both for the reply below and, in send, to pace any
	// later spontaneous pushes the dispatch produces.
	if g := protocol.GuardMillis(flags); g != 0 {
		n.guard = time.Duration(g) * time.Millisecond
	}
	// Reply turnaround guard (protocol/README.md §6): the hub tells us, in the request's
	// GUARD field, how long to wait before answering so its half-duplex radio has
	// finished switching from transmit back to receive. A fast MCU would otherwise
	// reply into a window when the hub is not yet listening and the reply would be
	// lost. This is measured from the request's arrival (the hub's transmit→receive
	// switch); send adds the separate spacing between consecutive transmits.
	if n.guard != 0 {
		time.Sleep(n.guard)
	}
	switch typ {
	case protocol.TypeGET:
		v, null := n.dev.Read(reg)
		n.replyIS(src, reg, v, null)
	case protocol.TypeSET:
		// Acknowledge receipt first (protocol/README.md §8.2); the ACK carries no value.
		// It must go out before dev.Write because Write may push one or more IS
		// Notify packets to watchers (and itself settle asynchronously); sending
		// those first would delay the ACK past the hub's response timeout. The
		// eventual settled value reaches any watcher via Notify.
		n.send(src, protocol.TypeACK, 0, reg, 0)
		n.dev.Write(reg, value, flags&protocol.FlagNULL != 0)
	case protocol.TypeWATCH:
		if value != 0 {
			n.subscribe(src, reg)
			v, null := n.dev.Read(reg)
			n.replyIS(src, reg, v, null)
		} else {
			n.unsubscribe(src, reg)
		}
	case protocol.TypeACK:
		// The hub acknowledges a spontaneous push (protocol/README.md §8.3): the matching
		// in-flight push is delivered and stops retransmitting. (A node never
		// receives a SET ACK; those flow node→hub.)
		n.clearPush(src, reg)
	default:
		// IS is node→hub only; a node never receives one. Ignore anything else.
	}
	n.servicePending()
	return true
}

// Notify pushes the new value of a register to every hub currently watching it.
// Firmware calls this whenever a register's value changes on its own (a sensor
// reading, a relay toggled locally, …) so subscribers are kept up to date
// (protocol/README.md §8, WATCH). When null is true the register has become unset and
// the push carries the NULL flag with value 0 (the dual of a NULL IS reply).
//
// Unlike a solicited IS reply, a push has no hub request behind it to be
// retransmitted on loss, so it is sent reliably: the push is marked PUSH
// (FlagPush), transmitted immediately, and retransmitted by the runtime (in
// Poll) until the hub ACKs it or the attempt ceiling is reached.
func (n *Node) Notify(tag uint16, value int32, null bool) {
	for i := range n.subs {
		s := &n.subs[i]
		if s.active && s.tag == tag {
			n.startPush(s.addr, tag, value, null)
		}
	}
}

// startPush records (or refreshes) the pending push for (addr, tag) and sends it
// once now. The retransmit pump (servicePending) resends it until the hub ACKs.
func (n *Node) startPush(addr [4]byte, tag uint16, value int32, null bool) {
	p := n.pendingSlot(addr, tag)
	p.addr = addr
	p.tag = tag
	p.value = value
	p.null = null
	p.active = true
	p.tries = 1
	n.sendPush(p)
	p.deadline = nowMillis() + pushRetryMillis
}

// sendPush transmits the pending push as an IS marked PUSH (and NULL when unset).
func (n *Node) sendPush(p *pendingPush) {
	flags := protocol.FlagPush
	value := p.value
	if p.null {
		flags |= protocol.FlagNULL
		value = 0
	}
	n.send(p.addr, protocol.TypeIS, flags, p.tag, value)
}

// pendingSlot returns the pending-push entry for (addr, tag): the existing one if
// present, else a free slot, else the oldest (round-robin eviction) so the table
// never allocates and never overflows.
func (n *Node) pendingSlot(addr [4]byte, tag uint16) *pendingPush {
	var free *pendingPush
	for i := range n.pending {
		p := &n.pending[i]
		if p.active && p.addr == addr && p.tag == tag {
			return p
		}
		if free == nil && !p.active {
			free = p
		}
	}
	if free != nil {
		return free
	}
	p := &n.pending[n.nextPending]
	n.nextPending = (n.nextPending + 1) % maxPending
	return p
}

// servicePending retransmits every pending push whose ACK wait has elapsed, up
// to pushMaxTries, after which the push is abandoned. It runs on every Poll so a
// push keeps trying through the hub's transmit windows even when no packet is
// arriving. Deadlines are millisecond ticks (nowMillis); pushDue compares them
// wrap-safely.
func (n *Node) servicePending() {
	now := nowMillis()
	for i := range n.pending {
		p := &n.pending[i]
		if !p.active || !pushDue(now, p.deadline) {
			continue
		}
		if p.tries >= pushMaxTries {
			p.active = false
			continue
		}
		p.tries++
		n.sendPush(p)
		p.deadline = nowMillis() + pushRetryMillis
	}
}

// pushDue reports whether a retransmit scheduled for deadline is due at now.
// Both are nowMillis ticks; the signed difference keeps the comparison correct
// across the ~49-day uint32 wrap (a deadline just past the wrap is still seen as
// later than a now just before it).
func pushDue(now, deadline uint32) bool {
	return int32(now-deadline) >= 0
}

// nowMillis returns a monotonic millisecond tick used to schedule push
// retransmits. It wraps every ~49 days; callers compare deadlines with a signed
// difference (servicePending) so the wrap is harmless.
func nowMillis() uint32 {
	return uint32(time.Now().UnixNano() / int64(time.Millisecond))
}

// clearPush stops retransmitting the push for (addr, tag) once the hub ACKs it.
func (n *Node) clearPush(addr [4]byte, tag uint16) {
	for i := range n.pending {
		p := &n.pending[i]
		if p.active && p.addr == addr && p.tag == tag {
			p.active = false
			return
		}
	}
}

// replyIS sends a solicited IS reply to dst with the NULL flag set when null. It
// is fire-once (no PUSH flag, no ACK expected): a lost reply is recovered by the
// hub retransmitting its GET/WATCH request.
func (n *Node) replyIS(dst [4]byte, reg uint16, value int32, null bool) {
	var flags byte
	if null {
		flags = protocol.FlagNULL
		value = 0
	}
	n.send(dst, protocol.TypeIS, flags, reg, value)
}

// send encodes one packet into txBuf and hands it to the radio, pacing it behind
// the previous transmit by the hub's turnaround guard (protocol/README.md §6). The hub's
// half-duplex dongle needs that long to read one packet out and re-arm its
// receiver, so two packets sent back-to-back — a SET's ACK and the IS push the
// write produces, or two pushes from one change — would see the second arrive
// before the hub is listening again and be lost. Pacing is from the node's last
// transmit (the hub's receive readout), distinct from the reply guard in Poll
// (the hub's transmit→receive switch).
func (n *Node) send(dst [4]byte, typ, flags byte, reg uint16, value int32) {
	if n.guard != 0 {
		if wait := n.guard - time.Since(n.lastTx); wait > 0 {
			time.Sleep(wait)
		}
	}
	n.codec.Encode(n.txBuf[:], n.self, typ, flags, reg, value)
	_ = n.radio.Send(dst, n.txBuf[:])
	n.lastTx = time.Now()
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
