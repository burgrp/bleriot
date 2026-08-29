package bridge

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/burgrp/bleriot/lib/site/engine"
	"github.com/burgrp/bleriot/lib/site/node"
)

var nodeAddr = [4]byte{0xCC, 0xA0, 0x00, 0x02}

const (
	testChannel uint8  = 10
	testRegA    uint16 = 0x1234
	testRegB    uint16 = 0x5678
)

type getCall struct {
	addr [4]byte
	reg  uint16
	resp chan getResult
}

type getResult struct {
	update engine.Update
	err    error
}

type fakeTx struct {
	gets     chan getCall
	mu       sync.Mutex
	sets     []int32
	setNulls int
	setDone  chan struct{}
	setErr   error
}

func newFakeTx() *fakeTx {
	return &fakeTx{gets: make(chan getCall, 32), setDone: make(chan struct{}, 32)}
}

func (tx *fakeTx) Get(ctx context.Context, addr [4]byte, reg uint16) (engine.Update, error) {
	call := getCall{addr: addr, reg: reg, resp: make(chan getResult, 1)}
	select {
	case tx.gets <- call:
	case <-ctx.Done():
		return engine.Update{}, ctx.Err()
	}
	select {
	case result := <-call.resp:
		return result.update, result.err
	case <-ctx.Done():
		return engine.Update{}, ctx.Err()
	}
}

func (tx *fakeTx) Set(_ context.Context, _ [4]byte, _ uint16, value int32) error {
	tx.mu.Lock()
	tx.sets = append(tx.sets, value)
	err := tx.setErr
	tx.mu.Unlock()
	tx.setDone <- struct{}{}
	return err
}

func (tx *fakeTx) SetNull(context.Context, [4]byte, uint16) error {
	tx.mu.Lock()
	tx.setNulls++
	tx.mu.Unlock()
	tx.setDone <- struct{}{}
	return nil
}

func nextGet(t *testing.T, tx *fakeTx) getCall {
	t.Helper()
	select {
	case call := <-tx.gets:
		return call
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for GET")
		return getCall{}
	}
}

func answer(call getCall, value int32) { call.resp <- getResult{update: engine.Update{Value: value}} }

type providedRegister struct {
	initial  any
	metadata map[string]any
	updates  chan any
	requests chan any
}
type fakeRegistry struct {
	mu       sync.Mutex
	provided map[string]*providedRegister
	notify   chan string
	fail     map[string]error
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{provided: make(map[string]*providedRegister), notify: make(chan string, 32), fail: make(map[string]error)}
}

func (registry *fakeRegistry) Provide(_ context.Context, name string, initial any, metadata map[string]any, _ time.Duration) (chan<- any, <-chan any, error) {
	provided := &providedRegister{initial: initial, metadata: metadata, updates: make(chan any, 32), requests: make(chan any, 32)}
	registry.mu.Lock()
	if err := registry.fail[name]; err != nil {
		registry.mu.Unlock()
		return nil, nil, err
	}
	registry.provided[name] = provided
	registry.mu.Unlock()
	registry.notify <- name
	return provided.updates, provided.requests, nil
}

func (registry *fakeRegistry) waitProvided(t *testing.T, name string) *providedRegister {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		registry.mu.Lock()
		provided := registry.provided[name]
		registry.mu.Unlock()
		if provided != nil {
			return provided
		}
		select {
		case <-registry.notify:
		case <-deadline:
			t.Fatalf("register %q was not provided", name)
		}
	}
}

func receive(t *testing.T, channel <-chan any) any {
	t.Helper()
	select {
	case value := <-channel:
		return value
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Registry update")
		return nil
	}
}

func floatRegister(id uint16, name string) node.Register {
	return node.Register{ID: id, Name: name, Type: node.TypeFloat, Conversion: node.Conversion{
		Decode: func(raw int32) (any, error) { return float64(raw) / 100, nil },
		Encode: func(value any) (int32, error) {
			converted, ok := value.(float64)
			if !ok {
				return 0, fmt.Errorf("expected float64, got %T", value)
			}
			return int32(converted * 100), nil
		},
	}, Metadata: map[string]string{"unit": "celsius"}}
}

func testNode(t *testing.T, name string, address [4]byte, registers ...node.Register) *node.Node {
	t.Helper()
	descriptor, err := node.NewDescriptor(nil, registers)
	if err != nil {
		t.Fatal(err)
	}
	return node.NewNode(name, testChannel, descriptor, node.Identity{Address: address})
}

func newTestBridge(tx Transactor, registry Registry, interval time.Duration, threshold int) *Bridge {
	return New(tx, registry, time.Second, WithSweepInterval(interval), WithFailureThreshold(threshold))
}

func TestBridgeProvidesNilBeforeFirstGetCompletes(t *testing.T) {
	tx := newFakeTx()
	registry := newFakeRegistry()
	bridge := newTestBridge(tx, registry, time.Hour, 3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bridge.ServeNode(ctx, testNode(t, "test", nodeAddr, floatRegister(testRegA, "temperature")))
	call := nextGet(t, tx)
	provided := registry.waitProvided(t, "test.temperature")
	if provided.initial != nil {
		t.Fatalf("initial value = %v, want nil", provided.initial)
	}
	if provided.metadata["unit"] != "celsius" || provided.metadata["type"] != "float" || provided.metadata["device"] != "test" {
		t.Fatalf("metadata = %v", provided.metadata)
	}
	answer(call, 1234)
	if value := receive(t, provided.updates); value != 12.34 {
		t.Fatalf("GET value = %v, want 12.34", value)
	}
}

func TestBridgeProvideFailureDoesNotStopOtherRegistersOrPolling(t *testing.T) {
	tx := newFakeTx()
	registry := newFakeRegistry()
	registry.fail["test.a"] = errors.New("registry unavailable")
	bridge := newTestBridge(tx, registry, time.Hour, 3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bridge.ServeNode(ctx, testNode(t, "test", nodeAddr, floatRegister(testRegA, "a"), floatRegister(testRegB, "b")))
	providedB := registry.waitProvided(t, "test.b")

	first := nextGet(t, tx)
	if first.reg != testRegA {
		t.Fatalf("first GET register = %#x, want failed-provider register %#x", first.reg, testRegA)
	}
	answer(first, 100)
	second := nextGet(t, tx)
	if second.reg != testRegB {
		t.Fatalf("second GET register = %#x, want working register %#x", second.reg, testRegB)
	}
	answer(second, 250)
	if value := receive(t, providedB.updates); value != 2.5 {
		t.Fatalf("working register update = %v, want 2.5", value)
	}
}

type closedRequestRegistry struct{}

func (closedRequestRegistry) Provide(context.Context, string, any, map[string]any, time.Duration) (chan<- any, <-chan any, error) {
	updates := make(chan any)
	requests := make(chan any)
	close(requests)
	return updates, requests, nil
}

func TestServeRegisterExitsWhenRequestStreamCloses(t *testing.T) {
	bridge := newTestBridge(newFakeTx(), closedRequestRegistry{}, time.Hour, 3)
	n := testNode(t, "test", nodeAddr, floatRegister(testRegA, "a"))
	job := &polledNode{ctx: context.Background(), bridge: bridge, node: n}
	bridged := &bridgedRegister{register: &n.Registers[0], values: make(chan any, 1)}
	done := make(chan struct{})
	go func() {
		bridge.serveRegister(job.ctx, job, bridged)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("serveRegister did not exit after request stream closed")
	}
}

func TestBridgeFirstSweepDoesNotLeavePendingWake(t *testing.T) {
	tx := newFakeTx()
	registry := newFakeRegistry()
	bridge := newTestBridge(tx, registry, time.Hour, 3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bridge.ServeNode(ctx, testNode(t, "test", nodeAddr, floatRegister(testRegA, "temperature")))
	answer(nextGet(t, tx), 1234)

	select {
	case call := <-tx.gets:
		t.Fatalf("unexpected second initial sweep: addr=%v reg=%#x", call.addr, call.reg)
	case <-time.After(40 * time.Millisecond):
	}
}

func TestBridgeChannelSweepsAreSerializedCompleteAndRotating(t *testing.T) {
	tx := newFakeTx()
	registry := newFakeRegistry()
	bridge := newTestBridge(tx, registry, 20*time.Millisecond, 3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstAddr, secondAddr := [4]byte{1}, [4]byte{2}
	bridge.ServeNode(ctx, testNode(t, "first", firstAddr, floatRegister(testRegA, "a"), floatRegister(testRegB, "b")))
	first := nextGet(t, tx)
	bridge.ServeNode(ctx, testNode(t, "second", secondAddr, floatRegister(testRegA, "a"), floatRegister(testRegB, "b")))
	select {
	case unexpected := <-tx.gets:
		t.Fatalf("overlapping GET while first blocked: %+v", unexpected)
	case <-time.After(40 * time.Millisecond):
	}
	answer(first, 1)
	second := nextGet(t, tx)
	if second.addr != firstAddr || second.reg != testRegB {
		t.Fatalf("first sweep incomplete: addr=%v reg=%#x", second.addr, second.reg)
	}
	answer(second, 2)
	third := nextGet(t, tx)
	if third.addr != secondAddr || third.reg != testRegA {
		t.Fatalf("round did not rotate: addr=%v reg=%#x", third.addr, third.reg)
	}
	answer(third, 3)
	fourth := nextGet(t, tx)
	if fourth.addr != secondAddr || fourth.reg != testRegB {
		t.Fatalf("second sweep incomplete: addr=%v reg=%#x", fourth.addr, fourth.reg)
	}
	answer(fourth, 4)
	fifth := nextGet(t, tx)
	if fifth.addr != firstAddr || fifth.reg != testRegA {
		t.Fatalf("node repeated before complete round: addr=%v reg=%#x", fifth.addr, fifth.reg)
	}
	answer(fifth, 5)
}

func TestBridgeCancelFirstNodeDoesNotStopSharedChannel(t *testing.T) {
	tx := newFakeTx()
	registry := newFakeRegistry()
	bridge := newTestBridge(tx, registry, time.Hour, 3)
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	secondCtx, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()
	firstAddr, secondAddr := [4]byte{1}, [4]byte{2}
	bridge.ServeNode(firstCtx, testNode(t, "first", firstAddr, floatRegister(testRegA, "a")))
	first := nextGet(t, tx)
	bridge.ServeNode(secondCtx, testNode(t, "second", secondAddr, floatRegister(testRegA, "a")))
	cancelFirst()

	second := nextGet(t, tx)
	if second.addr != secondAddr {
		t.Fatalf("GET after first cancellation addressed %v, want %v", second.addr, secondAddr)
	}
	answer(second, 2)
	provided := registry.waitProvided(t, "second.a")
	if value := receive(t, provided.updates); value != 0.02 {
		t.Fatalf("second node value = %v, want 0.02", value)
	}
	select {
	case first.resp <- getResult{}:
	default:
	}
}

func TestBridgeCancelAllAndAddReplacementKeepsOnePoller(t *testing.T) {
	tx := newFakeTx()
	registry := newFakeRegistry()
	bridge := newTestBridge(tx, registry, time.Hour, 3)
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	secondCtx, cancelSecond := context.WithCancel(context.Background())
	bridge.ServeNode(firstCtx, testNode(t, "first", [4]byte{1}, floatRegister(testRegA, "a")))
	first := nextGet(t, tx)
	bridge.ServeNode(secondCtx, testNode(t, "second", [4]byte{2}, floatRegister(testRegA, "a")))
	cancelFirst()
	cancelSecond()

	replacementCtx, cancelReplacement := context.WithCancel(context.Background())
	defer cancelReplacement()
	replacementAddr := [4]byte{3}
	bridge.ServeNode(replacementCtx, testNode(t, "replacement", replacementAddr, floatRegister(testRegA, "a")))
	replacement := nextGet(t, tx)
	if replacement.addr == ([4]byte{2}) {
		// Cancellation may race the second job between its context check and
		// entering Get. Its canceled context makes that call return immediately.
		replacement = nextGet(t, tx)
	}
	if replacement.addr != replacementAddr {
		t.Fatalf("replacement GET addressed %v, want %v", replacement.addr, replacementAddr)
	}
	answer(replacement, 3)

	bridge.mu.Lock()
	if len(bridge.channels) != 1 {
		t.Fatalf("channel poller count = %d, want 1", len(bridge.channels))
	}
	poller := bridge.channels[testChannel]
	bridge.mu.Unlock()
	poller.mu.Lock()
	active := 0
	for _, job := range poller.nodes {
		if job.ctx.Err() == nil {
			active++
		}
	}
	poller.mu.Unlock()
	if active != 1 {
		t.Fatalf("active jobs = %d, want replacement only", active)
	}
	select {
	case first.resp <- getResult{}:
	default:
	}
}

func TestBridgeSetWaitsForNormalGetConfirmation(t *testing.T) {
	tx := newFakeTx()
	registry := newFakeRegistry()
	bridge := newTestBridge(tx, registry, 20*time.Millisecond, 3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bridge.ServeNode(ctx, testNode(t, "test", nodeAddr, floatRegister(testRegA, "temperature")))
	provided := registry.waitProvided(t, "test.temperature")
	initial := nextGet(t, tx)
	answer(initial, 1000)
	if value := receive(t, provided.updates); value != 10.0 {
		t.Fatalf("initial value = %v", value)
	}
	provided.requests <- 18.0
	select {
	case <-tx.setDone:
	case <-time.After(time.Second):
		t.Fatal("SET was not called")
	}
	select {
	case value := <-provided.updates:
		t.Fatalf("SET reflected optimistically as %v", value)
	case <-time.After(10 * time.Millisecond):
	}
	tx.mu.Lock()
	if len(tx.sets) != 1 || tx.sets[0] != 1800 {
		t.Fatalf("sets = %v, want [1800]", tx.sets)
	}
	tx.mu.Unlock()
	confirmation := nextGet(t, tx)
	answer(confirmation, 1750)
	if value := receive(t, provided.updates); value != 17.5 {
		t.Fatalf("confirmed value = %v, want 17.5", value)
	}
}

func TestBridgeSetNullWaitsForNormalGetConfirmation(t *testing.T) {
	tx := newFakeTx()
	registry := newFakeRegistry()
	bridge := newTestBridge(tx, registry, 20*time.Millisecond, 3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bridge.ServeNode(ctx, testNode(t, "test", nodeAddr, floatRegister(testRegA, "temperature")))
	provided := registry.waitProvided(t, "test.temperature")
	initial := nextGet(t, tx)
	answer(initial, 1000)
	receive(t, provided.updates)

	provided.requests <- nil
	select {
	case <-tx.setDone:
	case <-time.After(time.Second):
		t.Fatal("SET NULL was not called")
	}
	select {
	case value := <-provided.updates:
		t.Fatalf("SET NULL reflected optimistically as %v", value)
	case <-time.After(10 * time.Millisecond):
	}
	tx.mu.Lock()
	if tx.setNulls != 1 || len(tx.sets) != 0 {
		t.Fatalf("setNulls = %d, sets = %v", tx.setNulls, tx.sets)
	}
	tx.mu.Unlock()

	confirmation := nextGet(t, tx)
	confirmation.resp <- getResult{update: engine.Update{Null: true}}
	if value := receive(t, provided.updates); value != nil {
		t.Fatalf("confirmed NULL value = %v, want nil", value)
	}
}

func TestSetLivenessDependsOnAcknowledgement(t *testing.T) {
	for _, tt := range []struct {
		name         string
		setErr       error
		wantFailures int
		wantOffline  bool
		wantSuccess  uint64
	}{
		{name: "acknowledged", wantFailures: 0, wantOffline: false, wantSuccess: 1},
		{name: "failed", setErr: errors.New("timeout"), wantFailures: 2, wantOffline: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tx := newFakeTx()
			tx.setErr = tt.setErr
			bridge := newTestBridge(tx, newFakeRegistry(), time.Hour, 3)
			n := testNode(t, "test", nodeAddr, floatRegister(testRegA, "a"))
			job := &polledNode{ctx: context.Background(), bridge: bridge, node: n, failures: 2, offline: true}

			bridge.applyRequest(job.ctx, job, &n.Registers[0], 12.5, bridge.log)

			job.mu.Lock()
			failures, offline, successes := job.failures, job.offline, job.successes
			job.mu.Unlock()
			if failures != tt.wantFailures || offline != tt.wantOffline || successes != tt.wantSuccess {
				t.Fatalf("liveness = failures %d offline %v successes %d; want %d/%v/%d", failures, offline, successes, tt.wantFailures, tt.wantOffline, tt.wantSuccess)
			}
		})
	}
}

func TestDeliverLatestCoalescesWhileConsumerBlocked(t *testing.T) {
	values := make(chan any, 1)
	deliverLatest(values, 1)
	deliverLatest(values, 2)
	deliverLatest(values, 3)
	if value := <-values; value != 3 {
		t.Fatalf("coalesced value = %v, want latest value 3", value)
	}
	select {
	case value := <-values:
		t.Fatalf("obsolete value remained queued: %v", value)
	default:
	}
}

func TestBridgeFailedSweepsNilNodeOnceAndPollingRecovers(t *testing.T) {
	tx := newFakeTx()
	registry := newFakeRegistry()
	bridge := newTestBridge(tx, registry, 5*time.Millisecond, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bridge.ServeNode(ctx, testNode(t, "test", nodeAddr, floatRegister(testRegA, "a"), floatRegister(testRegB, "b")))
	a, b := registry.waitProvided(t, "test.a"), registry.waitProvided(t, "test.b")
	for _, value := range []int32{100, 200} {
		call := nextGet(t, tx)
		answer(call, value)
	}
	if receive(t, a.updates) != 1.0 || receive(t, b.updates) != 2.0 {
		t.Fatal("initial sweep values were not published")
	}
	for range 4 {
		call := nextGet(t, tx)
		call.resp <- getResult{err: engine.ErrTimeout}
	}
	if receive(t, a.updates) != nil || receive(t, b.updates) != nil {
		t.Fatal("offline sweep did not nil every register")
	}
	for range 2 {
		call := nextGet(t, tx)
		call.resp <- getResult{err: engine.ErrTimeout}
	}
	select {
	case value := <-a.updates:
		t.Fatalf("offline nil was republished: %v", value)
	case <-time.After(10 * time.Millisecond):
	}
	for _, value := range []int32{300, 400} {
		call := nextGet(t, tx)
		answer(call, value)
	}
	if receive(t, a.updates) != 3.0 || receive(t, b.updates) != 4.0 {
		t.Fatal("successful GETs did not recover values")
	}
}

func TestBridgeAnySuccessfulGetKeepsWholeNodeAvailable(t *testing.T) {
	tx := newFakeTx()
	registry := newFakeRegistry()
	bridge := newTestBridge(tx, registry, 5*time.Millisecond, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bridge.ServeNode(ctx, testNode(t, "test", nodeAddr, floatRegister(testRegA, "a"), floatRegister(testRegB, "b")))
	a, b := registry.waitProvided(t, "test.a"), registry.waitProvided(t, "test.b")
	for _, value := range []int32{100, 200} {
		call := nextGet(t, tx)
		answer(call, value)
	}
	receive(t, a.updates)
	receive(t, b.updates)

	for sweep := range 3 {
		call := nextGet(t, tx)
		answer(call, int32(300+sweep))
		call = nextGet(t, tx)
		call.resp <- getResult{err: engine.ErrTimeout}
		receive(t, a.updates)
	}
	select {
	case value := <-b.updates:
		t.Fatalf("partially responsive node was nulled: %v", value)
	case <-time.After(10 * time.Millisecond):
	}
}

func TestBridgeNullDecodeErrorAndReadOnlyRequests(t *testing.T) {
	tx := newFakeTx()
	registry := newFakeRegistry()
	readOnly := floatRegister(testRegA, "sensor")
	readOnly.ReadOnly = true
	readOnly.Conversion.Encode = nil
	readOnly.Conversion.Decode = func(int32) (any, error) { return nil, errors.New("invalid sample") }
	bridge := newTestBridge(tx, registry, time.Hour, 3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bridge.ServeNode(ctx, testNode(t, "test", nodeAddr, readOnly))
	provided := registry.waitProvided(t, "test.sensor")
	call := nextGet(t, tx)
	answer(call, 42)
	if receive(t, provided.updates) != nil {
		t.Fatal("decode error did not publish nil")
	}
	provided.requests <- 10.0
	provided.requests <- nil
	select {
	case <-tx.setDone:
		t.Fatal("read-only request issued SET")
	case <-time.After(20 * time.Millisecond):
	}
}
