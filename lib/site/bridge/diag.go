package bridge

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/burgrp/bleriot/lib/site/engine"
	"github.com/burgrp/bleriot/lib/site/node"
	"github.com/burgrp/bleriot/lib/site/radio"
	wirerest "github.com/burgrp/reg/pkg/wire/rest"
)

const (
	DefaultDiagnosticInterval = time.Second
	diagnosticSchemaVersion   = 2
)

// DiagnosticBatchRegistry is the Registry wire operation needed by the
// diagnostics publisher. *rest.ProviderClient satisfies it.
type DiagnosticBatchRegistry interface {
	SetRegisters(context.Context, map[string]wirerest.RegisterUpdate) error
}

// DiagNodeSource provides per-node diagnostic counters. *engine.Engine satisfies it.
type DiagNodeSource interface {
	SnapshotNode(addr [node.AddrLen]byte) engine.NodeStats
}

type DiagNode struct {
	Name string
	Addr [node.AddrLen]byte
}

type DiagDongle struct {
	Name  string
	Stats func() radio.DongleStats
}

type Diagnostics struct {
	src      DiagNodeSource
	reg      DiagnosticBatchRegistry
	prefix   string
	interval time.Duration
	ttl      time.Duration
	started  int64
	log      *slog.Logger

	mu                 sync.Mutex
	batchSuccess       uint64
	batchError         uint64
	valuesSent         uint64
	valuesCoalesced    uint64
	lastPublishSuccess int64
	lastPublishError   int64
}

type DiagOption func(*Diagnostics)

func WithDiagLogger(logger *slog.Logger) DiagOption {
	return func(diagnostics *Diagnostics) {
		if logger != nil {
			diagnostics.log = logger
		}
	}
}

// NewDiagnostics creates a cumulative diagnostics publisher. interval controls
// snapshot and batch publication cadence; ttl controls Registry expiry.
func NewDiagnostics(src DiagNodeSource, reg DiagnosticBatchRegistry, prefix string, interval, ttl time.Duration, opts ...DiagOption) *Diagnostics {
	if interval <= 0 {
		interval = DefaultDiagnosticInterval
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	diagnostics := &Diagnostics{
		src: src, reg: reg, prefix: strings.TrimSuffix(prefix, "."), interval: interval,
		ttl: ttl, started: time.Now().Unix(), log: slog.New(discardHandler{}),
	}
	for _, option := range opts {
		option(diagnostics)
	}
	return diagnostics
}

// Serve starts one publisher goroutine for the complete diagnostics catalog.
func (d *Diagnostics) Serve(ctx context.Context, nodes []DiagNode, dongles []DiagDongle) {
	nodes = append([]DiagNode(nil), nodes...)
	dongles = append([]DiagDongle(nil), dongles...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
	sort.Slice(dongles, func(i, j int) bool { return dongles[i].Name < dongles[j].Name })
	d.log.Info("serving batched diagnostics", "prefix", d.prefix, "nodes", len(nodes), "dongles", len(dongles), "interval", d.interval)
	go d.run(ctx, nodes, dongles)
}

type diagnosticValue struct {
	value any
	typ   string
}

func (d *Diagnostics) run(ctx context.Context, nodes []DiagNode, dongles []DiagDongle) {
	lastSent := make(map[string]any)
	pending := make(map[string]bool)
	tick := uint64(0)
	publish := func(now time.Time) {
		values := d.snapshot(nodes, dongles, now)
		names := sortedValueNames(values)
		cohortCount := d.refreshCohortCount()
		updates := make(map[string]wirerest.RegisterUpdate)
		for index, name := range names {
			value := values[name]
			previous, sent := lastSent[name]
			changed := !sent || !valuesEqual(previous, value.value)
			refresh := sent && uint64(index%cohortCount) == tick%uint64(cohortCount)
			if !changed && !refresh && !pending[name] {
				continue
			}
			metadata := map[string]any(nil)
			if !sent {
				metadata = diagMeta(value.typ)
			}
			updates[name] = wirerest.RegisterUpdate{Value: value.value, Metadata: metadata, TTL: d.ttl}
		}
		if len(updates) == 0 {
			tick++
			return
		}
		if err := d.reg.SetRegisters(ctx, updates); err != nil {
			for name := range updates {
				pending[name] = true
			}
			d.recordBatchError(now)
			d.log.Error("diagnostics batch publish failed", "values", len(updates), "err", err)
			tick++
			return
		}
		for name, update := range updates {
			lastSent[name] = update.Value
			delete(pending, name)
		}
		d.recordBatchSuccess(now, uint64(len(updates)), uint64(len(values)-len(updates)))
		tick++
	}

	publish(time.Now())
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			publish(now)
		}
	}
}

func (d *Diagnostics) refreshCohortCount() int {
	count := int(d.ttl / (2 * d.interval))
	if count < 1 {
		return 1
	}
	return count
}

func (d *Diagnostics) recordBatchSuccess(now time.Time, sent, coalesced uint64) {
	d.mu.Lock()
	d.batchSuccess++
	d.valuesSent += sent
	d.valuesCoalesced += coalesced
	d.lastPublishSuccess = now.Unix()
	d.mu.Unlock()
}

func (d *Diagnostics) recordBatchError(now time.Time) {
	d.mu.Lock()
	d.batchError++
	d.lastPublishError = now.Unix()
	d.mu.Unlock()
}

func (d *Diagnostics) publisherValues() map[string]diagnosticValue {
	d.mu.Lock()
	defer d.mu.Unlock()
	prefix := d.prefix + ".hub.main."
	return map[string]diagnosticValue{
		prefix + "schema.version":             integer(diagnosticSchemaVersion),
		prefix + "process.started":            integer(d.started),
		prefix + "publisher.batch.success":    integer(d.batchSuccess),
		prefix + "publisher.batch.error":      integer(d.batchError),
		prefix + "publisher.values.sent":      integer(d.valuesSent),
		prefix + "publisher.values.coalesced": integer(d.valuesCoalesced),
		prefix + "publisher.last.success":     integer(d.lastPublishSuccess),
		prefix + "publisher.last.error":       integer(d.lastPublishError),
	}
}

func (d *Diagnostics) snapshot(nodes []DiagNode, dongles []DiagDongle, now time.Time) map[string]diagnosticValue {
	values := d.publisherValues()
	values[d.prefix+".hub.main.process.heartbeat"] = integer(now.Unix())
	for _, diagNode := range nodes {
		addNodeValues(values, d.prefix, diagNode.Name, d.src.SnapshotNode(diagNode.Addr))
	}
	for _, dongle := range dongles {
		addDongleValues(values, d.prefix, dongle.Name, dongle.Stats())
	}
	return values
}

func addNodeValues(values map[string]diagnosticValue, prefix, name string, stats engine.NodeStats) {
	base := prefix + ".node." + pathComponent(name) + "."
	values[base+"liveness.state"] = integer(stats.Liveness.State)
	values[base+"liveness.since"] = integer(stats.Liveness.Since)
	values[base+"liveness.last.success"] = integer(stats.Liveness.LastSuccess)
	values[base+"liveness.last.failure"] = integer(stats.Liveness.LastFailure)
	values[base+"liveness.probe.misses"] = integer(stats.Liveness.Misses)
	values[base+"liveness.transition.online"] = integer(stats.Liveness.TransitionsOnline)
	values[base+"liveness.transition.suspect"] = integer(stats.Liveness.TransitionsSuspect)
	values[base+"liveness.transition.offline"] = integer(stats.Liveness.TransitionsOffline)

	packet := stats.Packet
	packetValues := map[string]uint64{
		"packet.rx.total": packet.RxTotal, "packet.rx.valid": packet.RxValid,
		"packet.rx.push_is": packet.RxPushIS, "packet.rx.solicited_is": packet.RxSolicitedIS,
		"packet.rx.orphan_is": packet.RxOrphanIS, "packet.rx.matched_ack": packet.RxMatchedACK,
		"packet.rx.orphan_ack": packet.RxOrphanACK, "packet.rx.null_is": packet.RxNullIS,
		"packet.rx.invalid.decode": packet.RxInvalidDecode, "packet.rx.invalid.source": packet.RxInvalidSource,
		"packet.rx.invalid.type": packet.RxInvalidType, "packet.rx.invalid.register": packet.RxUnknownRegister,
		"packet.tx.success": packet.TxSuccess, "packet.tx.error": packet.TxError,
		"packet.push_ack.success": packet.PushACKSuccess, "packet.push_ack.error": packet.PushACKError,
		"packet.push_ack.no_radio": packet.PushACKNoRadio,
	}
	for suffix, value := range packetValues {
		values[base+suffix] = integer(value)
	}
	values[base+"packet.last.received"] = integer(packet.LastReceived)
	values[base+"packet.last.valid"] = integer(packet.LastValid)

	for operation := engine.TransactionOperation(0); operation < engine.TransactionOperationCount; operation++ {
		transaction := stats.Transactions[operation]
		transactionBase := base + "transaction." + operation.String() + "."
		for outcome := engine.TransactionOutcome(0); outcome < engine.TransactionOutcomeCount; outcome++ {
			values[transactionBase+"outcome."+outcome.String()] = integer(transaction.Outcomes[outcome])
		}
		values[transactionBase+"attempt.initial"] = integer(transaction.AttemptInitial)
		values[transactionBase+"attempt.retry"] = integer(transaction.AttemptRetry)
		values[transactionBase+"attempt.send_error"] = integer(transaction.AttemptSendError)
		values[transactionBase+"attempt.response_timeout"] = integer(transaction.ResponseTimeout)
		values[transactionBase+"latency.success.count"] = integer(transaction.LatencyCount)
		values[transactionBase+"latency.success.microseconds"] = integer(transaction.LatencySumMicros)
	}
	values[base+"latency.success.count"] = integer(stats.Latency.Count)
	values[base+"latency.success.microseconds"] = integer(stats.Latency.SumMicros)
	for index, count := range stats.Latency.Buckets {
		label := strings.NewReplacer("+", "plus_", ".", "_").Replace(engine.LatencyBucketLabel(index))
		values[base+"latency.success.bucket.le_"+label] = integer(count)
	}
}

func addDongleValues(values map[string]diagnosticValue, prefix, name string, stats radio.DongleStats) {
	base := prefix + ".channel." + pathComponent(name) + "."
	values[base+"state"] = integer(stats.State)
	values[base+"state.since"] = integer(stats.StateSince)
	values[base+"connection.open.attempt"] = integer(stats.OpenAttempts)
	values[base+"connection.open.success"] = integer(stats.OpenSuccesses)
	values[base+"connection.open.error"] = integer(stats.OpenFailures)
	values[base+"connection.open.last_attempt"] = integer(stats.LastOpenAttempt)
	values[base+"connection.open.last_error"] = integer(stats.LastOpenFailure)
	values[base+"connection.connected_at"] = integer(stats.LastConnected)
	values[base+"connection.disconnected_at"] = integer(stats.LastDisconnected)
	values[base+"connection.disconnect.total"] = integer(stats.Disconnects)
	values[base+"connection.disconnect.send_error"] = integer(stats.DisconnectSendErrors)
	values[base+"connection.disconnect.receive_error"] = integer(stats.DisconnectReceiveErrors)
	values[base+"packet.tx.attempt"] = integer(stats.TxAttempts)
	values[base+"packet.tx.success"] = integer(stats.TxSuccess)
	values[base+"packet.tx.offline"] = integer(stats.TxOffline)
	values[base+"packet.tx.error"] = integer(stats.TxErrors)
	values[base+"packet.rx.success"] = integer(stats.RxSuccess)
	values[base+"packet.rx.error"] = integer(stats.RxErrors)
}

func integer(value any) diagnosticValue { return diagnosticValue{value: value, typ: "int"} }

func pathComponent(name string) string { return strings.ReplaceAll(name, ".", "_") }

func diagMeta(valueType string) map[string]any {
	return map[string]any{"type": valueType, "readOnly": true, "diagnostic": true, "schema": diagnosticSchemaVersion}
}

func sortedValueNames(values map[string]diagnosticValue) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func valuesEqual(left, right any) bool { return left == right }
