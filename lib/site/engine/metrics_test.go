package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/burgrp/bleriot/lib/shared/protocol"
)

func TestSnapshotNodeCountsGetValue(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 50*time.Millisecond, 1)
	go func() {
		request := <-radio.sent
		_, _, flags, reg, _, _ := codec.Decode(request[:])
		injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 1)
	}()
	if _, err := engine.Get(context.Background(), nodeAddr, regTemp); err != nil {
		t.Fatal(err)
	}

	stats := engine.SnapshotNode(nodeAddr)
	get := stats.Transactions[TransactionGet]
	if get.Outcomes[TransactionSuccessFirst] != 1 || get.AttemptInitial != 1 || get.AttemptRetry != 0 {
		t.Fatalf("GET stats = %+v", get)
	}
	if get.LatencyCount != 1 || stats.Latency.Count != 1 || stats.Latency.Buckets[LatencyBucketCount-1] != 1 {
		t.Fatalf("latency stats = %+v / %+v", get, stats.Latency)
	}
	if stats.Packet.RxMatchedVALUE != 1 || stats.Packet.RxMatchedACK != 0 || stats.Packet.TxSuccess != 1 {
		t.Fatalf("packet stats = %+v", stats.Packet)
	}
}

func TestSnapshotNodeCountsSetAck(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 50*time.Millisecond, 1)
	go func() {
		request := <-radio.sent
		_, _, flags, reg, _, _ := codec.Decode(request[:])
		injectResponse(radio, codec, nodeAddr, protocol.TypeACK, flags, reg, 0)
	}()
	if err := engine.Set(context.Background(), nodeAddr, regTemp, 5); err != nil {
		t.Fatal(err)
	}
	stats := engine.SnapshotNode(nodeAddr)
	if stats.Transactions[TransactionSet].Outcomes[TransactionSuccessFirst] != 1 {
		t.Fatalf("SET stats = %+v", stats.Transactions[TransactionSet])
	}
	if stats.Packet.RxMatchedACK != 1 || stats.Packet.RxMatchedVALUE != 0 {
		t.Fatalf("packet stats = %+v", stats.Packet)
	}
}

func TestSnapshotNodeCountsRetryAndTimeout(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 20*time.Millisecond, 1)
	radio.mu.Lock()
	radio.drop = 1
	radio.mu.Unlock()
	go func() {
		request := <-radio.sent
		_, _, flags, reg, _, _ := codec.Decode(request[:])
		injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 1)
	}()
	if _, err := engine.Get(context.Background(), nodeAddr, regTemp); err != nil {
		t.Fatal(err)
	}
	get := engine.SnapshotNode(nodeAddr).Transactions[TransactionGet]
	if get.Outcomes[TransactionSuccessRetry] != 1 || get.AttemptInitial != 1 || get.AttemptRetry != 1 || get.AttemptTimeout != 1 {
		t.Fatalf("retry GET stats = %+v", get)
	}

	timeoutEngine, _, _ := newTestEngine(t, 15*time.Millisecond, 1)
	if _, err := timeoutEngine.Get(context.Background(), nodeAddr, regTemp); !errors.Is(err, ErrTimeout) {
		t.Fatalf("Get error = %v, want timeout", err)
	}
	timedOut := timeoutEngine.SnapshotNode(nodeAddr)
	get = timedOut.Transactions[TransactionGet]
	if get.Outcomes[TransactionTimeout] != 1 || get.AttemptInitial != 1 || get.AttemptRetry != 1 || get.AttemptTimeout != 2 {
		t.Fatalf("timeout GET stats = %+v", get)
	}
	if timedOut.Packet.TxSuccess != 2 {
		t.Fatalf("timeout packet stats = %+v", timedOut.Packet)
	}
}

func TestSnapshotNodeCountsQueuedCancellation(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 200*time.Millisecond, 1)
	firstDone := make(chan error, 1)
	go func() { _, err := engine.Get(context.Background(), nodeAddr, regTemp); firstDone <- err }()
	request := <-radio.sent

	ctx, cancel := context.WithCancel(context.Background())
	queuedDone := make(chan error, 1)
	go func() { _, err := engine.Get(ctx, nodeAddr, regOther); queuedDone <- err }()
	time.Sleep(10 * time.Millisecond)
	cancel()
	if err := <-queuedDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("queued Get error = %v", err)
	}

	_, _, flags, reg, _, _ := codec.Decode(request[:])
	injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 1)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	get := engine.SnapshotNode(nodeAddr).Transactions[TransactionGet]
	if get.Outcomes[TransactionCanceled] != 1 || get.Outcomes[TransactionSuccessFirst] != 1 {
		t.Fatalf("queued cancellation stats = %+v", get)
	}
}

func TestSnapshotNodeUnknownIsZero(t *testing.T) {
	engine, _, _ := newTestEngine(t, 50*time.Millisecond, 1)
	if stats := engine.SnapshotNode([4]byte{9, 9, 9, 9}); stats != (NodeStats{}) {
		t.Fatalf("unknown node stats = %+v", stats)
	}
}
