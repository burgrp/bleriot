// Package bridge connects the BleRiot polling engine to the external Registry.
// One scheduler per RF channel interleaves register reads between nodes; SET
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

// WithSweepInterval sets the minimum start-to-start period of each node's full
// register sweep. Ready nodes take turns one register at a time.
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

	nextRegister int
	nextSweep    time.Time
	sweepStarted time.Time
	startedWith  uint64
	lastPoll     time.Time
	pollStats    PollStats
}

// PollStats describes actual polling cadence rather than transaction success.
type PollStats struct {
	LastStarted         int64
	LastGapMicros       int64
	MaxGapMicros        int64
	Sweeps              uint64
	LastCompleted       int64
	LastDurationMicros  int64
	ConsecutiveFailures int
}

// ServeNode registers n with its channel scheduler and returns immediately.
// Registry providers start at nil; the first sweep follows immediately.
func (bridge *Bridge) ServeNode(ctx context.Context, n *node.Node) {
	if len(n.Registers) == 0 {
		bridge.log.Warn("node has no registers to poll", "device", n.Name)
		return
	}
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
		job, wait := poller.next(time.Now())
		if wait < 0 {
			if poller.retireIfEmpty() {
				return
			}
			continue
		}
		if job != nil {
			job.pollNext()
			continue
		}
		timer := time.NewTimer(wait)
		select {
		case <-poller.wake:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
	}
}

// next gives each ready node one GET before visiting it again, including nodes
// with fewer registers. Completed sweeps wait without delaying other nodes.
func (poller *channelPoller) next(now time.Time) (*polledNode, time.Duration) {
	poller.mu.Lock()
	defer poller.mu.Unlock()
	active := poller.nodes[:0]
	for _, job := range poller.nodes {
		if job.ctx.Err() == nil {
			active = append(active, job)
		}
	}
	poller.nodes = active
	if len(poller.nodes) == 0 {
		return nil, -1
	}
	var wait time.Duration
	for offset := range poller.nodes {
		index := (poller.cursor + offset) % len(poller.nodes)
		job := poller.nodes[index]
		remaining := job.nextSweep.Sub(now)
		if remaining <= 0 {
			poller.cursor = index + 1
			return job, 0
		}
		if wait == 0 || remaining < wait {
			wait = remaining
		}
	}
	return nil, wait
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

func (job *polledNode) pollNext() {
	if job.ctx.Err() != nil {
		return
	}
	started := time.Now()
	if job.nextRegister == 0 {
		job.sweepStarted = started
		job.startedWith = job.successCount()
	}
	job.mu.Lock()
	if !job.lastPoll.IsZero() {
		job.pollStats.LastGapMicros = started.Sub(job.lastPoll).Microseconds()
		job.pollStats.MaxGapMicros = max(job.pollStats.MaxGapMicros, job.pollStats.LastGapMicros)
	}
	job.lastPoll = started
	job.pollStats.LastStarted = started.Unix()
	job.mu.Unlock()

	bridged := &job.registers[job.nextRegister]
	update, err := job.bridge.tx.Get(job.ctx, job.node.Address, bridged.register.ID)
	if err == nil {
		job.noteSuccess()
		value, err := regValue(bridged.register, update)
		if err != nil {
			job.bridge.log.Warn("failed to decode polled node value", "device", job.node.Name, "register", bridged.register.Name, "raw", update.Value, "err", err)
			value = nil
		}
		deliverLatest(bridged.values, value)
	}
	if job.ctx.Err() != nil {
		return
	}
	job.nextRegister++
	if job.nextRegister == len(job.registers) {
		job.nextRegister = 0
		job.nextSweep = job.sweepStarted.Add(job.bridge.sweepInterval)
		job.noteFailedSweep(job.startedWith)
		job.mu.Lock()
		job.pollStats.Sweeps++
		job.pollStats.LastCompleted = time.Now().Unix()
		job.pollStats.LastDurationMicros = time.Since(job.sweepStarted).Microseconds()
		job.mu.Unlock()
	}
}

// SnapshotPolling returns cadence and full-sweep failure statistics for a node.
func (bridge *Bridge) SnapshotPolling(addr [node.AddrLen]byte) PollStats {
	var found *polledNode
	bridge.mu.Lock()
	for _, poller := range bridge.channels {
		poller.mu.Lock()
		for _, job := range poller.nodes {
			if job.node.Address == addr {
				found = job
				break
			}
		}
		poller.mu.Unlock()
		if found != nil {
			break
		}
	}
	bridge.mu.Unlock()
	if found == nil {
		return PollStats{}
	}
	found.mu.Lock()
	defer found.mu.Unlock()
	stats := found.pollStats
	stats.ConsecutiveFailures = found.failures
	return stats
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
	name := register.RegistryName
	if name == "" {
		name = job.node.Name + "." + register.Name
	}
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
	result := make(map[string]any, len(register.Metadata)+5)
	for key, value := range register.Metadata {
		result[key] = value
	}
	result["type"] = string(register.Type)
	result["device"] = deviceName
	result["register"] = register.Name
	result["tag"] = register.ID
	result["readOnly"] = register.ReadOnly
	return result
}
