// Package bridge connects the BleRiot polling engine to the external Registry.
// One scheduler per RF channel performs serialized, complete node sweeps; SET
// acknowledgements are not reflected optimistically, so Registry values always
// come from a subsequent GET.
package bridge

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/burgrp/bleriot/lib/site/engine"
	"github.com/burgrp/bleriot/lib/site/node"
)

const (
	DefaultSweepInterval    = time.Second
	DefaultFailureThreshold = 3
)

// Transactor is the engine surface the bridge needs. *engine.Engine satisfies it.
type Transactor interface {
	Get(ctx context.Context, addr [node.AddrLen]byte, reg uint16) (engine.Update, error)
	Set(ctx context.Context, addr [node.AddrLen]byte, reg uint16, value int32) error
	SetNull(ctx context.Context, addr [node.AddrLen]byte, reg uint16) error
}

// Registry is the registry-client surface the bridge needs.
type Registry interface {
	Provide(ctx context.Context, name string, value any, metadata map[string]any, ttl time.Duration) (chan<- any, <-chan any, error)
}

// Bridge publishes node registers to a Registry and applies consumer changes.
type Bridge struct {
	tx               Transactor
	reg              Registry
	ttl              time.Duration
	sweepInterval    time.Duration
	failureThreshold int
	log              *slog.Logger

	mu       sync.Mutex
	channels map[uint8]*channelPoller
}

type Option func(*Bridge)

func WithLogger(logger *slog.Logger) Option {
	return func(bridge *Bridge) {
		if logger != nil {
			bridge.log = logger
		}
	}
}

// WithSweepInterval sets the target period between complete channel sweeps.
// Overloaded sweeps slip naturally and never overlap.
func WithSweepInterval(interval time.Duration) Option {
	return func(bridge *Bridge) {
		if interval > 0 {
			bridge.sweepInterval = interval
		}
	}
}

// WithFailureThreshold sets how many wholly unsuccessful sweeps make a node
// unavailable. Its registers are then published as nil once.
func WithFailureThreshold(threshold int) Option {
	return func(bridge *Bridge) {
		if threshold > 0 {
			bridge.failureThreshold = threshold
		}
	}
}

func New(tx Transactor, reg Registry, ttl time.Duration, opts ...Option) *Bridge {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	bridge := &Bridge{
		tx: tx, reg: reg, ttl: ttl,
		sweepInterval: DefaultSweepInterval, failureThreshold: DefaultFailureThreshold,
		log: slog.New(discardHandler{}), channels: make(map[uint8]*channelPoller),
	}
	for _, option := range opts {
		option(bridge)
	}
	return bridge
}

type discardHandler struct{}

func (discardHandler) Enabled(context.Context, slog.Level) bool   { return false }
func (discardHandler) Handle(context.Context, slog.Record) error  { return nil }
func (handler discardHandler) WithAttrs([]slog.Attr) slog.Handler { return handler }
func (handler discardHandler) WithGroup(string) slog.Handler      { return handler }

type bridgedRegister struct {
	register *node.Register
	values   chan any
}

type polledNode struct {
	ctx       context.Context
	bridge    *Bridge
	node      *node.Node
	registers []bridgedRegister

	successes uint64
	mu        sync.Mutex
	failures  int
	offline   bool
}

// ServeNode registers n with its channel scheduler and returns immediately.
// Registry providers start at nil; the first sweep follows immediately.
func (bridge *Bridge) ServeNode(ctx context.Context, n *node.Node) {
	job := &polledNode{ctx: ctx, bridge: bridge, node: n, registers: make([]bridgedRegister, len(n.Registers))}
	for index := range n.Registers {
		register := &n.Registers[index]
		job.registers[index] = bridgedRegister{register: register, values: make(chan any, 1)}
		go bridge.serveRegister(ctx, job, &job.registers[index])
	}

	bridge.mu.Lock()
	poller := bridge.channels[n.Channel]
	if poller == nil {
		poller = &channelPoller{
			bridge:  bridge,
			channel: n.Channel,
			wake:    make(chan struct{}, 1),
			nodes:   []*polledNode{job},
		}
		bridge.channels[n.Channel] = poller
		poller.watch(job)
		go poller.run()
	} else {
		poller.add(job)
	}
	bridge.mu.Unlock()
}

type channelPoller struct {
	bridge  *Bridge
	channel uint8
	wake    chan struct{}

	mu     sync.Mutex
	nodes  []*polledNode
	cursor int
}

func (poller *channelPoller) add(job *polledNode) {
	poller.mu.Lock()
	poller.nodes = append(poller.nodes, job)
	poller.mu.Unlock()
	poller.watch(job)
	poller.signal()
}

func (poller *channelPoller) watch(job *polledNode) {
	go func() {
		<-job.ctx.Done()
		poller.signal()
	}()
}

func (poller *channelPoller) signal() {
	select {
	case poller.wake <- struct{}{}:
	default:
	}
}

func (poller *channelPoller) run() {
	for {
		started := time.Now()
		nodes, start := poller.round()
		if len(nodes) == 0 {
			if poller.retireIfEmpty() {
				return
			}
			continue
		}
		for offset := range nodes {
			job := nodes[(start+offset)%len(nodes)]
			if job.ctx.Err() == nil {
				job.sweep()
			}
		}

		remaining := poller.bridge.sweepInterval - time.Since(started)
		if remaining <= 0 {
			continue
		}
		timer := time.NewTimer(remaining)
		select {
		case <-poller.wake:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
	}
}

// round rotates the first node. Every snapshotted node completes before any is
// repeated, including when the channel cannot meet the target interval.
func (poller *channelPoller) round() ([]*polledNode, int) {
	poller.mu.Lock()
	defer poller.mu.Unlock()
	active := poller.nodes[:0]
	for _, job := range poller.nodes {
		if job.ctx.Err() == nil {
			active = append(active, job)
		}
	}
	poller.nodes = active
	nodes := append([]*polledNode(nil), poller.nodes...)
	if len(nodes) == 0 {
		return nodes, 0
	}
	start := poller.cursor % len(nodes)
	poller.cursor = start + 1
	return nodes, start
}

func (poller *channelPoller) retireIfEmpty() bool {
	poller.bridge.mu.Lock()
	poller.mu.Lock()
	retired := poller.bridge.channels[poller.channel] == poller && len(poller.nodes) == 0
	if retired {
		delete(poller.bridge.channels, poller.channel)
	}
	poller.mu.Unlock()
	poller.bridge.mu.Unlock()
	return retired
}

func (job *polledNode) sweep() {
	startedWith := job.successCount()
	for index := range job.registers {
		if job.ctx.Err() != nil {
			return
		}
		bridged := &job.registers[index]
		update, err := job.bridge.tx.Get(job.ctx, job.node.Address, bridged.register.ID)
		if err != nil {
			continue
		}
		job.noteSuccess()
		value, err := regValue(bridged.register, update)
		if err != nil {
			job.bridge.log.Warn("failed to decode polled node value", "device", job.node.Name, "register", bridged.register.Name, "raw", update.Value, "err", err)
			value = nil
		}
		deliverLatest(bridged.values, value)
	}
	if job.ctx.Err() == nil {
		job.noteFailedSweep(startedWith)
	}
}

func (job *polledNode) noteSuccess() {
	job.mu.Lock()
	job.successes++
	job.failures = 0
	job.offline = false
	job.mu.Unlock()
}

func (job *polledNode) successCount() uint64 {
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.successes
}

func (job *polledNode) noteFailedSweep(startedWith uint64) {
	job.mu.Lock()
	if job.successes != startedWith {
		job.mu.Unlock()
		return
	}
	job.failures++
	if job.failures < job.bridge.failureThreshold || job.offline {
		job.mu.Unlock()
		return
	}
	job.offline = true
	job.mu.Unlock()
	for index := range job.registers {
		deliverLatest(job.registers[index].values, nil)
	}
}

func (bridge *Bridge) serveRegister(ctx context.Context, job *polledNode, bridged *bridgedRegister) {
	register := bridged.register
	name := job.node.Name + "." + register.Name
	log := bridge.log.With("device", job.node.Name, "register", name, "id", register.ID)
	updates, requests, err := bridge.reg.Provide(ctx, name, nil, metadata(register, job.node.Name), bridge.ttl)
	if err != nil {
		log.Error("registry provide failed", "err", err)
		return
	}
	log.Info("providing register to registry")
	for {
		select {
		case <-ctx.Done():
			return
		case value := <-bridged.values:
			select {
			case updates <- value:
			case <-ctx.Done():
				return
			}
		case request, ok := <-requests:
			if !ok {
				return
			}
			bridge.applyRequest(ctx, job, register, request, log)
		}
	}
}

func (bridge *Bridge) applyRequest(ctx context.Context, job *polledNode, register *node.Register, request any, log *slog.Logger) {
	if register.ReadOnly {
		log.Warn("ignoring change request for read-only register", "value", request)
		return
	}
	var err error
	if request == nil {
		err = bridge.tx.SetNull(ctx, job.node.Address, register.ID)
	} else {
		var wire int32
		wire, err = register.Conversion.Encode(request)
		if err == nil {
			err = bridge.tx.Set(ctx, job.node.Address, register.ID, wire)
		}
	}
	if err != nil {
		log.Warn("SET failed", "value", request, "err", err)
		return
	}
	job.noteSuccess()
}

func deliverLatest(channel chan any, value any) {
	select {
	case channel <- value:
		return
	default:
	}
	select {
	case <-channel:
	default:
	}
	select {
	case channel <- value:
	default:
	}
}

func regValue(register *node.Register, update engine.Update) (any, error) {
	if update.Null {
		return nil, nil
	}
	return register.Conversion.Decode(update.Value)
}

func metadata(register *node.Register, deviceName string) map[string]any {
	result := make(map[string]any, len(register.Metadata)+3)
	for key, value := range register.Metadata {
		result[key] = value
	}
	result["type"] = string(register.Type)
	result["device"] = deviceName
	result["readOnly"] = register.ReadOnly
	return result
}
