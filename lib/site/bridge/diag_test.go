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
	registry.batches = append(registry.batches, cloneBatch(updates))
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

func TestDiagnosticsPublishesPollingSchema(t *testing.T) {
	if diagnosticSchemaVersion != 8 {
		t.Fatalf("diagnostic schema = %d, want 8", diagnosticSchemaVersion)
	}
	source := &fakeSnap{}
	get := &source.stats.Transactions[engine.TransactionGet]
	get.Outcomes[engine.TransactionSuccessFirst] = 7
	get.Outcomes[engine.TransactionTimeout] = 2
	get.AttemptInitial, get.AttemptRetry, get.AttemptTimeout = 9, 3, 5
	get.LatencyCount, get.LatencySumMicros = 7, 140000
	set := &source.stats.Transactions[engine.TransactionSet]
	set.Outcomes[engine.TransactionSuccessRetry] = 1
	set.AttemptInitial, set.AttemptRetry, set.LatencyCount, set.LatencySumMicros = 1, 1, 1, 30000
	source.stats.Packet = engine.PacketStats{
		RxMatchedVALUE: 7, RxOrphanVALUE: 2, RxNullVALUE: 1,
		RxMatchedACK: 1, RxOrphanACK: 3,
		RxInvalidDecode: 1, RxInvalidType: 3,
		LastReceived: 1700000000, LastValid: 1700000001,
	}
	source.stats.Latency = engine.LatencyStats{Count: 8, SumMicros: 170000, Buckets: [engine.LatencyBucketCount]uint64{1, 2, 3, 4, 5, 6, 7, 8}}
	registry := newFakeBatchRegistry()
	diagnostics := NewDiagnostics(source, registry, "diag", time.Hour, 30*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	diagnostics.Serve(ctx,
		[]DiagNode{{Name: "basement.fan", Addr: [4]byte{1, 2, 3, 4}}},
		[]DiagDongle{{Name: "far", Stats: func() radio.DongleStats {
			return radio.DongleStats{State: radio.DongleConnected, OpenAttempts: 3, TxErrors: 1}
		}}},
	)

	batch := registry.waitBatch(t)
	wants := map[string]any{
		"diag.hub.main.schema.version":                                        8,
		"diag.hub.main.latency.success.bucket.le_plus_Inf":                    uint64(8),
		"diag.node.basement_fan.transaction.get.outcome.success_first":        uint64(7),
		"diag.node.basement_fan.transaction.get.outcome.timeout":              uint64(2),
		"diag.node.basement_fan.transaction.get.attempt.retry":                uint64(3),
		"diag.node.basement_fan.transaction.get.latency.success.microseconds": uint64(140000),
		"diag.node.basement_fan.transaction.set.outcome.success_retry":        uint64(1),
		"diag.node.basement_fan.packet.value.matched":                         uint64(7),
		"diag.node.basement_fan.packet.value.orphan":                          uint64(2),
		"diag.node.basement_fan.packet.value.null":                            uint64(1),
		"diag.node.basement_fan.packet.ack.matched":                           uint64(1),
		"diag.node.basement_fan.packet.last.received":                         int64(1700000000),
		"diag.channel.far.connection.open.attempt":                            uint64(3),
		"diag.channel.far.packet.tx.error":                                    uint64(1),
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
		if update.Metadata["schema"] != 8 || update.TTL != 30*time.Second {
			t.Errorf("%s metadata/TTL = %v/%v", name, update.Metadata, update.TTL)
		}
	}
	nodePrefix := "diag.node.basement_fan."
	nodeRegisters := 0
	for name := range batch {
		if strings.HasPrefix(name, nodePrefix) {
			nodeRegisters++
		}
		for _, stale := range []string{".liveness.", ".push", ".transaction.poll", ".transaction.watch", ".transaction.all"} {
			if strings.Contains(name, stale) {
				t.Errorf("legacy diagnostic path published: %s", name)
			}
		}
	}
	if nodeRegisters != 33 {
		t.Errorf("node register count = %d, want 33", nodeRegisters)
	}
}

func TestPathComponentAvoidsDotUnderscoreCollisions(t *testing.T) {
	if pathComponent("a.b") == pathComponent("a_b") {
		t.Fatal("dotted and underscored names collapse to the same path")
	}
}

func TestDiagnosticsCoalescesUnchangedValuesAndRetriesFailure(t *testing.T) {
	registry := newFakeBatchRegistry()
	registry.failNext = true
	diagnostics := NewDiagnostics(&fakeSnap{}, registry, "diag", 5*time.Millisecond, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	diagnostics.Serve(ctx, []DiagNode{{Name: "lab", Addr: [4]byte{1}}}, nil)
	first := registry.waitBatch(t)
	if len(first) < 10 {
		t.Fatalf("retry batch contains %d values", len(first))
	}
	for _, update := range first {
		if update.Metadata == nil {
			t.Fatal("retry omitted metadata")
		}
	}
	second := registry.waitBatch(t)
	if len(second) >= len(first) {
		t.Fatalf("unchanged batch contains %d values, initial %d", len(second), len(first))
	}
	if _, ok := second["diag.hub.main.publisher.batch.success"]; !ok {
		t.Error("publisher success counter missing")
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
		t.Fatal("timed out waiting for failed refresh")
	}
	registry.mu.Lock()
	failed := cloneBatch(registry.failed[len(registry.failed)-1])
	registry.mu.Unlock()
	retried := registry.waitBatch(t)
	for name := range failed {
		if _, ok := retried[name]; !ok {
			t.Errorf("failed refresh %s was not retried", name)
		}
	}
}

func TestDiagMetaMarksRegisterReadOnly(t *testing.T) {
	metadata := diagMeta("int")
	if metadata["type"] != "int" || metadata["diagnostic"] != true || metadata["readOnly"] != true || metadata["schema"] != 8 {
		t.Fatalf("metadata = %v", metadata)
	}
}
