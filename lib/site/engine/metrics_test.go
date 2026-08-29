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

func TestUnknownNodeAndKnownNodeWithoutRadioAccounting(t *testing.T) {
	engine := New(Options{HubAddr: hubAddr, Timeout: 20 * time.Millisecond, Retries: 1})
	addTestNode(t, engine, "node", testChannel, nodeAddr, testKey)
	unknown := [4]byte{9, 9, 9, 9}
	if _, err := engine.Get(context.Background(), unknown, regTemp); !errors.Is(err, ErrUnknownNode) {
		t.Fatalf("unknown Get error = %v, want ErrUnknownNode", err)
	}
	if _, err := engine.Get(context.Background(), nodeAddr, regTemp); !errors.Is(err, ErrNoRadio) {
		t.Fatalf("known Get error = %v, want ErrNoRadio", err)
	}
	if stats := engine.SnapshotNode(unknown); stats != (NodeStats{}) {
		t.Fatalf("unknown node stats = %+v", stats)
	}
	get := engine.SnapshotNode(nodeAddr).Transactions[TransactionGet]
	if get.Outcomes[TransactionNoRadio] != 1 || get.AttemptInitial != 0 || get.AttemptRetry != 0 {
		t.Fatalf("known no-radio stats = %+v", get)
	}
}

func TestInitialSendErrorAccounting(t *testing.T) {
	engine, radio, _ := newTestEngine(t, 20*time.Millisecond, 1)
	want := errors.New("send failed")
	radio.mu.Lock()
	radio.sendErr = want
	radio.mu.Unlock()
	if _, err := engine.Get(context.Background(), nodeAddr, regTemp); !errors.Is(err, want) {
		t.Fatalf("Get error = %v, want wrapped send error", err)
	}
	stats := engine.SnapshotNode(nodeAddr)
	get := stats.Transactions[TransactionGet]
	if get.Outcomes[TransactionSendError] != 1 || get.AttemptInitial != 1 || get.AttemptRetry != 0 || get.AttemptSendError != 1 || get.AttemptTimeout != 0 {
		t.Fatalf("initial send-error stats = %+v", get)
	}
	if stats.Packet.TxSuccess != 0 || stats.Packet.TxError != 1 {
		t.Fatalf("initial send-error packet stats = %+v", stats.Packet)
	}
}

func TestSendErrorAfterTimeoutDrainsResponse(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 20*time.Millisecond, 1)
	want := errors.New("retry send failed")
	done := make(chan error, 1)
	go func() {
		_, err := engine.Get(context.Background(), nodeAddr, regTemp)
		done <- err
	}()
	request := <-radio.sent
	_, _, flags, reg, _, _ := codec.Decode(request[:])
	radio.mu.Lock()
	radio.sendErr = want
	radio.mu.Unlock()
	<-radio.receivedCalls
	<-radio.receivedCalls
	injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 7)
	if err := <-done; !errors.Is(err, want) {
		t.Fatalf("Get error = %v, want wrapped retry send error", err)
	}

	stats := engine.SnapshotNode(nodeAddr)
	get := stats.Transactions[TransactionGet]
	if get.Outcomes[TransactionSendError] != 1 || get.AttemptInitial != 1 || get.AttemptRetry != 1 || get.AttemptSendError != 1 || get.AttemptTimeout != 1 {
		t.Fatalf("retry send-error stats = %+v", get)
	}
	if stats.Packet.TxSuccess != 1 || stats.Packet.TxError != 1 || stats.Packet.RxOrphanVALUE != 1 {
		t.Fatalf("retry send-error packet stats = %+v", stats.Packet)
	}
}

func TestPacketClassificationAccounting(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 80*time.Millisecond, 0)
	unknownAddr := [4]byte{9, 8, 7, 6}
	unknownReg := uint16(0x9999)

	go func() {
		request := <-radio.sent
		_, _, flags, _, _, _ := codec.Decode(request[:])
		injectResponse(radio, codec, unknownAddr, protocol.TypeVALUE, flags, regTemp, 1)

		var invalidVersion [PacketLen]byte
		codec.Encode(invalidVersion[:], nodeAddr, protocol.TypeVALUE, flags, regTemp, 2)
		invalidVersion[4] = 0xff
		radio.recv <- invalidVersion
		injectResponse(radio, codec, nodeAddr, protocol.TypeGET, flags, regTemp, 3)
		injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags|protocol.FlagNULL, unknownReg, 0)
		injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags|protocol.FlagNULL, regTemp, 0)
	}()
	update, err := engine.Get(context.Background(), nodeAddr, regTemp)
	if err != nil || !update.Null {
		t.Fatalf("Get NULL = %+v, %v", update, err)
	}

	go func() {
		request := <-radio.sent
		_, _, flags, reg, _, _ := codec.Decode(request[:])
		injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 44)
	}()
	update, err = engine.Get(context.Background(), nodeAddr, unknownReg)
	if err != nil || update != (Update{Value: 44}) {
		t.Fatalf("Get unknown register = %+v, %v", update, err)
	}

	if stats := engine.SnapshotNode(unknownAddr); stats != (NodeStats{}) {
		t.Fatalf("unknown source stats = %+v", stats)
	}
	packet := engine.SnapshotNode(nodeAddr).Packet
	if packet.RxTotal != 5 || packet.RxValid != 3 || packet.RxInvalidDecode != 1 || packet.RxInvalidType != 1 || packet.RxUnknownRegister != 2 {
		t.Fatalf("validation packet stats = %+v", packet)
	}
	if packet.RxMatchedVALUE != 2 || packet.RxOrphanVALUE != 1 || packet.RxNullVALUE != 2 {
		t.Fatalf("VALUE packet stats = %+v", packet)
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
