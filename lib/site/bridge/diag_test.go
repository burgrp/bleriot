package bridge

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/burgrp/bleriot/lib/site/engine"
	"github.com/burgrp/bleriot/lib/site/radio"
	wirerest "github.com/burgrp/reg/pkg/wire/rest"
)

type fakeBatchRegistry struct {
	mu            sync.Mutex
	batches       []map[string]wirerest.RegisterUpdate
	failed        []map[string]wirerest.RegisterUpdate
	failNext      bool
	notify        chan struct{}
	failureNotify chan struct{}
}

func newFakeBatchRegistry() *fakeBatchRegistry {
	return &fakeBatchRegistry{notify: make(chan struct{}, 16), failureNotify: make(chan struct{}, 16)}
}

func (registry *fakeBatchRegistry) SetRegisters(_ context.Context, updates map[string]wirerest.RegisterUpdate) error {
	registry.mu.Lock()
	if registry.failNext {
		registry.failNext = false
		registry.failed = append(registry.failed, cloneBatch(updates))
		registry.mu.Unlock()
		registry.failureNotify <- struct{}{}
		return errors.New("unavailable")
	}
	batch := cloneBatch(updates)
	registry.batches = append(registry.batches, batch)
	registry.mu.Unlock()
	select {
	case registry.notify <- struct{}{}:
	default:
	}
	return nil
}

func cloneBatch(updates map[string]wirerest.RegisterUpdate) map[string]wirerest.RegisterUpdate {
	batch := make(map[string]wirerest.RegisterUpdate, len(updates))
	for name, update := range updates {
		batch[name] = update
	}
	return batch
}

func (registry *fakeBatchRegistry) waitBatch(t *testing.T) map[string]wirerest.RegisterUpdate {
	t.Helper()
	select {
	case <-registry.notify:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for diagnostics batch")
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	return registry.batches[len(registry.batches)-1]
}

type fakeSnap struct{ stats engine.NodeStats }

func (source *fakeSnap) SnapshotNode([4]byte) engine.NodeStats { return source.stats }

func TestDiagnosticsPublishesCompactSchema(t *testing.T) {
	source := &fakeSnap{}
	source.stats.Liveness = engine.LivenessStats{State: engine.LivenessOnline, Since: 1700000000, Misses: 1, TransitionsOnline: 2}
	source.stats.Packet = engine.PacketStats{RxTotal: 12, RxValid: 11, TxSuccess: 20, TxError: 1, LastValid: 1700000001}
	source.stats.Transactions[engine.TransactionGet].Outcomes[engine.TransactionSuccessFirst] = 7
	source.stats.Transactions[engine.TransactionGet].Outcomes[engine.TransactionTimeout] = 2
	source.stats.Transactions[engine.TransactionGet].AttemptRetry = 3
	source.stats.Packet.RxOrphanIS = 2
	source.stats.Packet.RxOrphanACK = 3
	source.stats.Packet.RxInvalidDecode = 1
	source.stats.Packet.RxInvalidType = 2
	source.stats.Packet.RxUnknownRegister = 4
	source.stats.Packet.PushACKError = 1
	source.stats.Packet.PushACKNoRadio = 2
	source.stats.Latency = engine.LatencyStats{Count: 7, SumMicros: 140000, Buckets: [engine.LatencyBucketCount]uint64{1, 2, 3, 4, 5, 6, 7, 7}}
	registry := newFakeBatchRegistry()
	diagnostics := NewDiagnostics(source, registry, "diag", time.Hour, 30*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	diagnostics.Serve(ctx,
		[]DiagNode{{Name: "basement.fan", Addr: [4]byte{1, 2, 3, 4}}},
		[]DiagDongle{{Name: "far", Stats: func() radio.DongleStats {
			return radio.DongleStats{State: radio.DongleConnected, OpenAttempts: 3, OpenSuccesses: 2, TxAttempts: 10, TxSuccess: 8, TxOffline: 1, TxErrors: 1}
		}}},
	)

	batch := registry.waitBatch(t)
	wants := map[string]any{
		"diag.hub.main.schema.version":                                 diagnosticSchemaVersion,
		"diag.hub.main.latency.success.bucket.le_plus_Inf":             uint64(7),
		"diag.node.basement_fan.liveness.state":                        engine.LivenessOnline,
		"diag.node.basement_fan.packet.rx.valid":                       uint64(11),
		"diag.node.basement_fan.packet.rx.orphan":                      uint64(5),
		"diag.node.basement_fan.packet.rx.invalid":                     uint64(3),
		"diag.node.basement_fan.packet.rx.unknown_register":            uint64(4),
		"diag.node.basement_fan.packet.push_ack.failure":               uint64(3),
		"diag.node.basement_fan.transaction.all.outcome.success_first": uint64(7),
		"diag.node.basement_fan.transaction.all.outcome.timeout":       uint64(2),
		"diag.node.basement_fan.transaction.get.invocation.total":      uint64(9),
		"diag.node.basement_fan.transaction.all.attempt.retry":         uint64(3),
		"diag.node.basement_fan.latency.success.microseconds":          uint64(140000),
		"diag.channel.far.connection.open.attempt":                     uint64(3),
		"diag.channel.far.packet.tx.error":                             uint64(1),
	}
	for name, want := range wants {
		update, ok := batch[name]
		if !ok {
			t.Errorf("missing %s", name)
			continue
		}
		if update.Value != want {
			t.Errorf("%s = %v (%T), want %v (%T)", name, update.Value, update.Value, want, want)
		}
		if update.Metadata["schema"] != diagnosticSchemaVersion || update.TTL != 30*time.Second {
			t.Errorf("%s metadata/TTL = %v/%v", name, update.Metadata, update.TTL)
		}
	}
	if _, old := batch["diag.node.basement_fan.rate.tx.all"]; old {
		t.Error("legacy rate register was published")
	}
	nodePrefix := "diag.node.basement_fan."
	nodeRegisters := 0
	for name := range batch {
		if strings.HasPrefix(name, nodePrefix) {
			nodeRegisters++
		}
	}
	if nodeRegisters != 29 {
		t.Errorf("node register count = %d, want 29", nodeRegisters)
	}
	if _, detailed := batch["diag.node.basement_fan.transaction.get.outcome.timeout"]; detailed {
		t.Error("operation-specific outcome leaked into compact schema")
	}
	if _, bucket := batch["diag.node.basement_fan.latency.success.bucket.le_plus_Inf"]; bucket {
		t.Error("per-node latency bucket leaked into compact schema")
	}
}

func TestPathComponentAvoidsDotUnderscoreCollisions(t *testing.T) {
	if pathComponent("a.b") == pathComponent("a_b") {
		t.Fatal("dotted and underscored names collapse to the same path")
	}
}

func TestDiagnosticsCoalescesUnchangedValuesAndRetriesFailure(t *testing.T) {
	source := &fakeSnap{}
	registry := newFakeBatchRegistry()
	registry.failNext = true
	diagnostics := NewDiagnostics(source, registry, "diag", 5*time.Millisecond, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	diagnostics.Serve(ctx, []DiagNode{{Name: "lab", Addr: [4]byte{1}}}, nil)

	firstSuccessful := registry.waitBatch(t)
	if len(firstSuccessful) < 10 {
		t.Fatalf("retry batch contains %d values, want complete initial catalog", len(firstSuccessful))
	}
	for _, update := range firstSuccessful {
		if update.Metadata == nil {
			t.Fatal("retry of failed initial batch omitted metadata")
		}
	}

	second := registry.waitBatch(t)
	if len(second) >= len(firstSuccessful) {
		t.Fatalf("unchanged batch contains %d values, want fewer than initial %d", len(second), len(firstSuccessful))
	}
	if _, ok := second["diag.hub.main.publisher.batch.success"]; !ok {
		t.Error("publisher success counter was not published on the next tick")
	}
}

func TestDiagnosticsRetriesFailedRefreshCohort(t *testing.T) {
	registry := newFakeBatchRegistry()
	diagnostics := NewDiagnostics(&fakeSnap{}, registry, "diag", 5*time.Millisecond, 20*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	diagnostics.Serve(ctx, []DiagNode{{Name: "lab", Addr: [4]byte{1}}}, nil)

	registry.waitBatch(t)
	registry.mu.Lock()
	registry.failNext = true
	registry.mu.Unlock()

	select {
	case <-registry.failureNotify:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for failed refresh cohort")
	}
	registry.mu.Lock()
	failed := cloneBatch(registry.failed[len(registry.failed)-1])
	registry.mu.Unlock()

	// The next successful batch must contain every name from the failed cohort,
	// even where that unchanged name belongs to the other normal TTL cohort.
	retried := registry.waitBatch(t)
	for name := range failed {
		if _, ok := retried[name]; !ok {
			t.Errorf("failed refresh value %s was not retried immediately", name)
		}
	}
}

func TestDiagMetaMarksRegisterReadOnly(t *testing.T) {
	metadata := diagMeta("int")
	if metadata["type"] != "int" || metadata["diagnostic"] != true || metadata["readOnly"] != true {
		t.Fatalf("diagnostic metadata = %v", metadata)
	}
}
