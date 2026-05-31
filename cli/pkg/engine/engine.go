// Package engine implements the BleRiot hub protocol logic (PROTOCOL.md §8–§10):
// GET/SET/WATCH transactions with best-effort retries and timeouts, response
// correlation, and push-subscription bookkeeping.
//
// The engine is transport-agnostic: it talks to radios through the small Radio
// interface (satisfied by *modem.Modem) and identifies nodes by their address
// using per-node XTEA codecs. Routing to a radio is by node channel; routing of
// received packets to a node is by source address.
package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"cli/pkg/node"
	"protocol"
)

// PacketLen is the fixed BleRiot on-wire packet size (§4).
const PacketLen = protocol.PacketLen

// Defaults from PROTOCOL.md §9.
const (
	DefaultTimeout = 50 * time.Millisecond
	DefaultRetries = 3
)

// DefaultRefreshInterval is how often Run re-sends WATCH for active
// subscriptions. A node drops a subscription after T_idle of silence from the
// hub (PROTOCOL.md §10, recommended 60 s), so the default leaves comfortable
// margin below that.
const DefaultRefreshInterval = 15 * time.Second

// DefaultLivenessMisses is how many consecutive unanswered WATCH refreshes
// (§10) mark a node offline. When a subscription's refresh has timed out this
// many times in a row, the engine delivers a NULL update to its watcher so a
// vanished node's last value is not reported indefinitely. With
// DefaultRefreshInterval this detects a powered-off node in ~30 s while
// tolerating a single lost refresh.
const DefaultLivenessMisses = 2

// Radio is the minimal transmit/receive surface the engine needs. *modem.Modem
// satisfies it.
type Radio interface {
	Send(dst [node.AddrLen]byte, payload []byte) error
	Received() <-chan [PacketLen]byte
}

// Update is an observed register value delivered to a Watch callback.
type Update struct {
	Value int32
	Null  bool // register has no value (FLAGS.NULL set)
}

// Callback receives subscription updates for a watched register.
type Callback func(Update)

var (
	// ErrTimeout is returned when no matching response arrives within the
	// configured timeout across all retries.
	ErrTimeout = errors.New("engine: transaction timed out")
	// ErrBusy is returned when another transaction for the same (node, register)
	// is already in flight.
	ErrBusy = errors.New("engine: transaction already in flight for register")
	// ErrUnknownNode is returned when no node is registered for an address.
	ErrUnknownNode = errors.New("engine: unknown node address")
	// ErrNoRadio is returned when no radio is registered for a node's channel.
	ErrNoRadio = errors.New("engine: no radio for node channel")
)

type key struct {
	addr [node.AddrLen]byte
	reg  uint16
}

// pendingReq is an in-flight transaction awaiting its reply. want is the reply
// TYPE that resolves it (IS for GET/WATCH, ACK for SET), so an unrelated push
// (e.g. a WATCH IS) cannot complete a pending SET.
type pendingReq struct {
	ch   chan Update
	want byte
}

type nodeState struct {
	n     *node.Node
	codec protocol.Codec
}

// Options configures an Engine.
type Options struct {
	HubAddr [node.AddrLen]byte // SRC address used in outgoing packets
	Timeout time.Duration      // per-attempt response wait (default §9)
	Retries int                // retransmissions after the first attempt (default §9)
	// RefreshInterval is how often Run re-WATCHes active subscriptions to keep
	// them alive within the node's T_idle window (§10). Defaults to
	// DefaultRefreshInterval.
	RefreshInterval time.Duration
	// LivenessMisses is the number of consecutive unanswered WATCH refreshes
	// after which a node is considered offline and its watchers receive a NULL
	// update. Defaults to DefaultLivenessMisses.
	LivenessMisses int
}

// Engine coordinates BleRiot transactions across radios and nodes.
type Engine struct {
	hubAddr  [node.AddrLen]byte
	timeout  time.Duration
	retries  int
	refresh  time.Duration
	liveness int

	mu      sync.Mutex
	radios  map[uint8]Radio // by channel
	nodes   map[[node.AddrLen]byte]*nodeState
	pending map[key]pendingReq
	subs    map[key]Callback
	misses  map[key]int // consecutive unanswered WATCH refreshes per subscription
}

// New creates an Engine. HubAddr should match the receive address configured on
// each radio so nodes' replies reach this hub.
func New(opts Options) *Engine {
	to := opts.Timeout
	if to <= 0 {
		to = DefaultTimeout
	}
	rt := opts.Retries
	if rt <= 0 {
		rt = DefaultRetries
	}
	rf := opts.RefreshInterval
	if rf <= 0 {
		rf = DefaultRefreshInterval
	}
	lm := opts.LivenessMisses
	if lm <= 0 {
		lm = DefaultLivenessMisses
	}
	return &Engine{
		hubAddr:  opts.HubAddr,
		timeout:  to,
		retries:  rt,
		refresh:  rf,
		liveness: lm,
		radios:   make(map[uint8]Radio),
		nodes:    make(map[[node.AddrLen]byte]*nodeState),
		pending:  make(map[key]pendingReq),
		subs:     make(map[key]Callback),
		misses:   make(map[key]int),
	}
}

// Run keeps active push subscriptions alive by periodically re-sending WATCH
// for every watched register. A node silently drops a subscription after T_idle
// of no packets from this hub (§10); re-WATCHing well within that window keeps
// pushes flowing and re-establishes subscriptions the node may have lost (e.g.
// after a reboot). Run blocks until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) {
	ticker := time.NewTicker(e.refresh)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.refreshSubscriptions(ctx)
		}
	}
}

// refreshSubscriptions re-WATCHes every active subscription. It is best-effort:
// transient errors (timeouts, an in-flight transaction for the same register)
// are ignored and retried on the next tick. The registered callback is
// preserved across refreshes.
func (e *Engine) refreshSubscriptions(ctx context.Context) {
	e.mu.Lock()
	keys := make([]key, 0, len(e.subs))
	for k := range e.subs {
		keys = append(keys, k)
	}
	e.mu.Unlock()

	for _, k := range keys {
		if ctx.Err() != nil {
			return
		}
		// Skip if the subscription was cancelled between snapshot and now.
		e.mu.Lock()
		_, live := e.subs[k]
		e.mu.Unlock()
		if !live {
			continue
		}
		_, err := e.transact(ctx, k.addr, protocol.TypeWATCH, 0, k.reg, 1)
		e.noteLiveness(k, err)
	}
}

// noteLiveness records the outcome of a subscription refresh and detects a node
// going offline. A successful refresh (or any received IS, see handle) clears
// the miss counter; consecutive timeouts accumulate, and exactly when they reach
// the liveness threshold the watcher is told the register is NULL so a vanished
// node's stale value stops being reported. ErrBusy is not a liveness signal (a
// transaction was merely already in flight) and is ignored.
func (e *Engine) noteLiveness(k key, err error) {
	if errors.Is(err, ErrBusy) {
		return
	}
	e.mu.Lock()
	cb, live := e.subs[k]
	if !live {
		delete(e.misses, k)
		e.mu.Unlock()
		return
	}
	if err == nil {
		e.misses[k] = 0
		e.mu.Unlock()
		return
	}
	e.misses[k]++
	cross := e.misses[k] == e.liveness
	e.mu.Unlock()
	if cross {
		cb(Update{Null: true})
	}
}

// AddRadio registers a radio for a channel and starts servicing its received
// packets until ctx is cancelled or the radio's channel closes.
func (e *Engine) AddRadio(ctx context.Context, channel uint8, r Radio) {
	e.mu.Lock()
	e.radios[channel] = r
	e.mu.Unlock()
	go e.recvLoop(ctx, r)
}

// AddNode registers a node, building its XTEA codec from the provisioned key.
func (e *Engine) AddNode(n *node.Node) error {
	c, err := protocol.NewCodec(n.Key)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.nodes[n.Address] = &nodeState{n: n, codec: c}
	e.mu.Unlock()
	return nil
}

// Get reads a register's current value (§8.1).
func (e *Engine) Get(ctx context.Context, addr [node.AddrLen]byte, reg uint16) (Update, error) {
	return e.transact(ctx, addr, protocol.TypeGET, 0, reg, 0)
}

// Set writes a register (§8.2). The node replies with an ACK that confirms
// receipt but carries no value; the resulting value is observed via a Watch
// subscription (or a subsequent Get). Set returns once the ACK arrives, or
// ErrTimeout after exhausting retries.
func (e *Engine) Set(ctx context.Context, addr [node.AddrLen]byte, reg uint16, value int32) error {
	_, err := e.transact(ctx, addr, protocol.TypeSET, 0, reg, value)
	return err
}

// SetNull clears a register (§8.2): it sends a SET with FLAGS.NULL=1, asking the
// node to unset the register. Like Set it returns once the ACK arrives.
func (e *Engine) SetNull(ctx context.Context, addr [node.AddrLen]byte, reg uint16) error {
	_, err := e.transact(ctx, addr, protocol.TypeSET, protocol.FlagNULL, reg, 0)
	return err
}

// Watch subscribes to a register (§8.3). cb is invoked for the immediate reply
// and on every subsequent pushed change until Unwatch.
//
// The subscription is a persistent intent: it is registered immediately and
// retained even if the initial WATCH attempt times out (e.g. the node is
// offline at startup). Run keeps re-sending WATCH until the node answers, so a
// node that comes up later is subscribed automatically. Watch returns the error
// from the first attempt for the caller's information, but the subscription
// stays active regardless.
func (e *Engine) Watch(ctx context.Context, addr [node.AddrLen]byte, reg uint16, cb Callback) error {
	if cb == nil {
		return errors.New("engine: nil callback")
	}
	k := key{addr, reg}
	e.mu.Lock()
	e.subs[k] = cb
	e.mu.Unlock()

	_, err := e.transact(ctx, addr, protocol.TypeWATCH, 0, reg, 1)
	return err
}

// Unwatch cancels a subscription (§8.4).
func (e *Engine) Unwatch(ctx context.Context, addr [node.AddrLen]byte, reg uint16) error {
	k := key{addr, reg}
	_, err := e.transact(ctx, addr, protocol.TypeWATCH, 0, reg, 0)
	e.mu.Lock()
	delete(e.subs, k)
	delete(e.misses, k)
	e.mu.Unlock()
	return err
}

func (e *Engine) transact(ctx context.Context, addr [node.AddrLen]byte, typ, flags byte, reg uint16, value int32) (Update, error) {
	e.mu.Lock()
	ns, ok := e.nodes[addr]
	if !ok {
		e.mu.Unlock()
		return Update{}, ErrUnknownNode
	}
	r, ok := e.radios[ns.n.Channel]
	if !ok {
		e.mu.Unlock()
		return Update{}, ErrNoRadio
	}
	k := key{addr, reg}
	if _, busy := e.pending[k]; busy {
		e.mu.Unlock()
		return Update{}, ErrBusy
	}
	ch := make(chan Update, 1)
	want := protocol.TypeIS
	if typ == protocol.TypeSET {
		want = protocol.TypeACK
	}
	e.pending[k] = pendingReq{ch: ch, want: want}
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.pending, k)
		e.mu.Unlock()
	}()

	var pkt [PacketLen]byte
	ns.codec.Encode(pkt[:], e.hubAddr, typ, flags, reg, value)

	for attempt := 0; attempt <= e.retries; attempt++ {
		if err := r.Send(addr, pkt[:]); err != nil {
			return Update{}, fmt.Errorf("engine: send: %w", err)
		}
		timer := time.NewTimer(e.timeout)
		select {
		case u := <-ch:
			timer.Stop()
			return u, nil
		case <-timer.C:
			// retransmit
		case <-ctx.Done():
			timer.Stop()
			return Update{}, ctx.Err()
		}
	}
	return Update{}, ErrTimeout
}

func (e *Engine) recvLoop(ctx context.Context, r Radio) {
	in := r.Received()
	for {
		select {
		case <-ctx.Done():
			return
		case pkt, ok := <-in:
			if !ok {
				return
			}
			e.handle(pkt)
		}
	}
}

func (e *Engine) handle(pkt [PacketLen]byte) {
	var src [node.AddrLen]byte
	copy(src[:], pkt[0:node.AddrLen])

	e.mu.Lock()
	ns, ok := e.nodes[src]
	e.mu.Unlock()
	if !ok {
		return // unknown source: silently discard (§5)
	}

	srcDec, typ, flags, reg, value, err := ns.codec.Decode(pkt[:])
	if err != nil || srcDec != src {
		return
	}

	switch typ {
	case protocol.TypeIS:
		// A value report: resolves a pending GET/WATCH and feeds subscribers.
	case protocol.TypeACK:
		// A SET acknowledgement: resolves the pending SET only. It carries no
		// value, so clear value/flags and never invoke a subscription callback.
		value, flags = 0, 0
	default:
		return // nodes send only IS and ACK
	}

	u := Update{Value: value, Null: flags&protocol.FlagNULL != 0}
	k := key{src, reg}

	e.mu.Lock()
	cb := e.subs[k]
	pr := e.pending[k]
	// A received IS proves the node is alive: clear any accumulated refresh
	// misses so an active node is never marked offline.
	if typ == protocol.TypeIS {
		if _, live := e.subs[k]; live {
			e.misses[k] = 0
		}
	}
	e.mu.Unlock()

	// ACKs are not value reports; they must not be delivered to a watcher.
	if cb != nil && typ == protocol.TypeIS {
		cb(u)
	}
	// Resolve a pending transaction only with the reply type it is waiting for,
	// so a WATCH push (IS) cannot complete a pending SET (ACK) and vice versa.
	if pr.ch != nil && pr.want == typ {
		select {
		case pr.ch <- u:
		default:
		}
	}
}
