// Package engine implements the host side of the BleRiot request/response
// protocol. Each RF channel is a half-duplex transaction group: one GET or SET,
// including retries and its reply wait, owns the channel at a time.
package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/burgrp/bleriot/lib/shared/protocol"
	"github.com/burgrp/bleriot/lib/site/node"
)

// PacketLen is the fixed BleRiot on-wire packet size.
const PacketLen = protocol.PacketLen

const (
	DefaultTimeout = 50 * time.Millisecond
	DefaultRetries = 3

	// A final timeout has already allowed a complete reply window. Keep ownership
	// briefly afterward so a reply delayed in the host receive path cannot satisfy
	// the next transaction for the same source and register.
	timeoutQuarantine = 10 * time.Millisecond
	minReplyHeadroom  = 10 * time.Millisecond
)

// Radio is the minimal transmit/receive surface the engine needs. *radio.Radio
// satisfies it.
type Radio interface {
	Send(dst [node.AddrLen]byte, payload []byte) error
	Received() <-chan [PacketLen]byte
	ReplyGuard() time.Duration
}

// Update is a register value returned by Get.
type Update struct {
	Value int32
	Null  bool
}

var (
	ErrTimeout       = errors.New("engine: transaction timed out")
	ErrUnknownNode   = errors.New("engine: unknown node address")
	ErrNoRadio       = errors.New("engine: no radio for node channel")
	ErrGuardTooLarge = errors.New("engine: radio reply guard too large for timeout")
)

type nodeState struct {
	n     *node.Node
	codec protocol.Codec
}

// channelState persists when a channel's physical radio is replaced, ensuring
// old and new Radio instances cannot create independent transaction lanes.
type channelState struct {
	gate chan struct{}

	mu    sync.RWMutex
	radio Radio
}

func newChannelState(r Radio) *channelState {
	state := &channelState{gate: make(chan struct{}, 1), radio: r}
	state.gate <- struct{}{}
	return state
}

func (state *channelState) setRadio(r Radio) {
	state.mu.Lock()
	state.radio = r
	state.mu.Unlock()
}

func (state *channelState) getRadio() Radio {
	state.mu.RLock()
	r := state.radio
	state.mu.RUnlock()
	return r
}

// Options configures an Engine.
type Options struct {
	HubAddr [node.AddrLen]byte
	Timeout time.Duration
	// Retries is the number of retransmissions after the initial attempt. Zero
	// means one attempt total; production callers should pass DefaultRetries.
	Retries int
}

// Engine coordinates BleRiot transactions across channel radios and nodes.
type Engine struct {
	hubAddr [node.AddrLen]byte
	timeout time.Duration
	retries int

	mu       sync.Mutex
	channels map[uint8]*channelState
	nodes    map[[node.AddrLen]byte]*nodeState
	metrics  map[[node.AddrLen]byte]*nodeMetrics
}

// New creates an Engine. HubAddr must match the receive address configured on
// each radio so node replies return to this hub.
func New(opts Options) *Engine {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	retries := opts.Retries
	if retries < 0 {
		retries = 0
	}
	return &Engine{
		hubAddr:  opts.HubAddr,
		timeout:  timeout,
		retries:  retries,
		channels: make(map[uint8]*channelState),
		nodes:    make(map[[node.AddrLen]byte]*nodeState),
		metrics:  make(map[[node.AddrLen]byte]*nodeMetrics),
	}
}

// AddRadio registers or replaces the one half-duplex radio for channel.
func (e *Engine) AddRadio(_ context.Context, channel uint8, r Radio) error {
	if guard := r.ReplyGuard(); guard+minReplyHeadroom > e.timeout {
		return fmt.Errorf("%w: guard %v + headroom %v exceeds timeout %v on channel %d",
			ErrGuardTooLarge, guard, minReplyHeadroom, e.timeout, channel)
	}
	e.mu.Lock()
	state := e.channels[channel]
	if state == nil {
		e.channels[channel] = newChannelState(r)
	} else {
		state.setRadio(r)
	}
	e.mu.Unlock()
	return nil
}

// AddNode registers a node, building its XTEA codec from the provisioned key.
func (e *Engine) AddNode(n *node.Node) error {
	codec, err := protocol.NewCodec(n.Key)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.nodes[n.Address] = &nodeState{n: n, codec: codec}
	e.metrics[n.Address] = &nodeMetrics{}
	e.mu.Unlock()
	return nil
}

// Get reads a register's current value.
func (e *Engine) Get(ctx context.Context, addr [node.AddrLen]byte, reg uint16) (Update, error) {
	return e.transact(ctx, TransactionGet, addr, protocol.TypeGET, 0, reg, 0)
}

// Set requests a register assignment and returns after the node acknowledges
// receiving it. The resulting state may settle asynchronously.
func (e *Engine) Set(ctx context.Context, addr [node.AddrLen]byte, reg uint16, value int32) error {
	_, err := e.transact(ctx, TransactionSet, addr, protocol.TypeSET, 0, reg, value)
	return err
}

// SetNull requests a register clear and returns after the node acknowledges
// receiving it. The resulting state may settle asynchronously.
func (e *Engine) SetNull(ctx context.Context, addr [node.AddrLen]byte, reg uint16) error {
	_, err := e.transact(ctx, TransactionSet, addr, protocol.TypeSET, protocol.FlagNULL, reg, 0)
	return err
}

func (e *Engine) transact(ctx context.Context, operation TransactionOperation, addr [node.AddrLen]byte, typ, flags byte, reg uint16, value int32) (Update, error) {
	started := time.Now()
	e.mu.Lock()
	ns := e.nodes[addr]
	if ns == nil {
		e.mu.Unlock()
		return Update{}, ErrUnknownNode
	}
	channel := e.channels[ns.n.Channel]
	nm := e.metricsFor(addr)
	e.mu.Unlock()
	if channel == nil {
		nm.recordOutcome(operation, TransactionNoRadio)
		return Update{}, ErrNoRadio
	}

	if err := ctx.Err(); err != nil {
		nm.recordOutcome(operation, TransactionCanceled)
		return Update{}, err
	}
	select {
	case <-channel.gate:
		defer func() { channel.gate <- struct{}{} }()
	case <-ctx.Done():
		nm.recordOutcome(operation, TransactionCanceled)
		return Update{}, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		nm.recordOutcome(operation, TransactionCanceled)
		return Update{}, err
	}

	r := channel.getRadio()
	if r == nil {
		nm.recordOutcome(operation, TransactionNoRadio)
		return Update{}, ErrNoRadio
	}
	want := protocol.TypeVALUE
	if typ == protocol.TypeSET {
		want = protocol.TypeACK
	}
	flags = protocol.FlagsWithGuard(flags, guardMillis(r.ReplyGuard()))
	var packet [PacketLen]byte
	ns.codec.Encode(packet[:], e.hubAddr, typ, flags, reg, value)

	sent := false
	for attempt := 0; attempt <= e.retries; attempt++ {
		if err := ctx.Err(); err != nil {
			if sent {
				e.drain(r)
			}
			nm.recordOutcome(operation, TransactionCanceled)
			return Update{}, err
		}
		if attempt == 0 {
			nm.transactions[operation].attemptInitial.Add(1)
		} else {
			nm.transactions[operation].attemptRetry.Add(1)
		}
		if err := r.Send(addr, packet[:]); err != nil {
			nm.transactions[operation].attemptSendError.Add(1)
			nm.recordOutcome(operation, TransactionSendError)
			nm.packet.txError.Add(1)
			if sent {
				e.drain(r)
			}
			return Update{}, fmt.Errorf("engine: send: %w", err)
		}
		sent = true
		nm.packet.txSuccess.Add(1)

		update, matched, err := e.waitForReply(ctx, r, addr, reg, want)
		if err != nil {
			e.drain(r)
			nm.recordOutcome(operation, TransactionCanceled)
			return Update{}, err
		}
		if matched {
			outcome := TransactionSuccessFirst
			if attempt > 0 {
				outcome = TransactionSuccessRetry
			}
			nm.recordOutcome(operation, outcome)
			nm.recordSuccessLatency(operation, time.Since(started))
			if attempt > 0 {
				e.drain(r)
			}
			return update, nil
		}
		nm.transactions[operation].attemptTimeout.Add(1)
	}

	e.drain(r)
	if err := ctx.Err(); err != nil {
		nm.recordOutcome(operation, TransactionCanceled)
		return Update{}, err
	}
	nm.recordOutcome(operation, TransactionTimeout)
	return Update{}, ErrTimeout
}

func (e *Engine) waitForReply(ctx context.Context, r Radio, addr [node.AddrLen]byte, reg uint16, want byte) (Update, bool, error) {
	timer := time.NewTimer(e.timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return Update{}, false, ctx.Err()
		case <-timer.C:
			return Update{}, false, nil
		case packet, ok := <-r.Received():
			if !ok {
				return Update{}, false, nil
			}
			if update, matched := e.inspectPacket(packet, addr, reg, want, true); matched {
				return update, true, nil
			}
		}
	}
}

// drain consumes packets for a bounded interval while the channel is still
// owned. The radio guard plus reply headroom covers a response that starts at
// the latest valid turnaround; timeoutQuarantine covers host receive latency.
// Caller cancellation cannot release the tokenless lane while such a response
// may still arrive.
func (e *Engine) drain(r Radio) {
	interval := timeoutQuarantine
	if radioWindow := r.ReplyGuard() + minReplyHeadroom; radioWindow > interval {
		interval = radioWindow
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			return
		case packet, ok := <-r.Received():
			if !ok {
				return
			}
			e.inspectPacket(packet, [node.AddrLen]byte{}, 0, 0, false)
		}
	}
}

// inspectPacket authenticates and accounts for a response. It matches only the
// expected source, register, and response type while the channel owner is
// waiting; every other valid response remains an orphan.
func (e *Engine) inspectPacket(packet [PacketLen]byte, expectedAddr [node.AddrLen]byte, expectedReg uint16, expectedType byte, accept bool) (Update, bool) {
	var clearSource [node.AddrLen]byte
	copy(clearSource[:], packet[:node.AddrLen])

	e.mu.Lock()
	ns := e.nodes[clearSource]
	nm := e.metricsFor(clearSource)
	e.mu.Unlock()
	if ns == nil {
		return Update{}, false
	}

	now := time.Now()
	nm.packet.rxTotal.Add(1)
	nm.packet.lastReceived.Store(now.Unix())
	_, typ, flags, reg, value, err := ns.codec.Decode(packet[:])
	if err != nil {
		nm.packet.rxInvalidDecode.Add(1)
		return Update{}, false
	}
	if typ != protocol.TypeVALUE && typ != protocol.TypeACK {
		nm.packet.rxInvalidType.Add(1)
		return Update{}, false
	}
	if _, known := ns.n.ByID(reg); !known {
		nm.packet.rxUnknownRegister.Add(1)
	}
	nm.packet.rxValid.Add(1)
	nm.packet.lastValid.Store(now.Unix())

	matched := accept && clearSource == expectedAddr && reg == expectedReg && typ == expectedType
	switch typ {
	case protocol.TypeVALUE:
		if flags&protocol.FlagNULL != 0 {
			nm.packet.rxNullVALUE.Add(1)
		}
		if matched {
			nm.packet.rxMatchedVALUE.Add(1)
		} else {
			nm.packet.rxOrphanVALUE.Add(1)
		}
	case protocol.TypeACK:
		if matched {
			nm.packet.rxMatchedACK.Add(1)
		} else {
			nm.packet.rxOrphanACK.Add(1)
		}
	}
	return Update{Value: value, Null: flags&protocol.FlagNULL != 0}, matched
}

func guardMillis(duration time.Duration) byte {
	milliseconds := duration.Milliseconds()
	if milliseconds <= 0 {
		return 0
	}
	if milliseconds > int64(protocol.MaxGuardMillis) {
		return protocol.MaxGuardMillis
	}
	return byte(milliseconds)
}
