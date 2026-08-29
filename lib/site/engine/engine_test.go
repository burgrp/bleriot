package engine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/burgrp/bleriot/lib/shared/protocol"
	"github.com/burgrp/bleriot/lib/site/node"
)

var (
	testKey   = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	testKey2  = [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	hubAddr   = [4]byte{0xCC, 0xA0, 0x00, 0x01}
	nodeAddr  = [4]byte{0xCC, 0xA0, 0x00, 0x02}
	nodeAddr2 = [4]byte{0xCC, 0xA0, 0x00, 0x03}
)

const (
	testChannel  uint8 = 10
	testChannel2 uint8 = 20
	regTemp            = uint16(0x1234)
	regOther           = uint16(0x4321)
)

func testDescriptor(t *testing.T) *node.Descriptor {
	t.Helper()
	descriptor, err := node.NewDescriptor(nil, []node.Register{
		{ID: regTemp, Name: "temperature", Type: node.TypeInt},
		{ID: regOther, Name: "other", Type: node.TypeInt},
	})
	if err != nil {
		t.Fatal(err)
	}
	return descriptor
}

type fakeRadio struct {
	sent          chan [PacketLen]byte
	recv          chan [PacketLen]byte
	receivedCalls chan struct{}
	guard         time.Duration

	mu      sync.Mutex
	drop    int
	sendErr error
}

func newFakeRadio() *fakeRadio {
	return &fakeRadio{
		sent:          make(chan [PacketLen]byte, 32),
		recv:          make(chan [PacketLen]byte, 32),
		receivedCalls: make(chan struct{}, 32),
	}
}

func (radio *fakeRadio) Send(_ [4]byte, payload []byte) error {
	radio.mu.Lock()
	drop := radio.drop > 0
	if drop {
		radio.drop--
	}
	err := radio.sendErr
	radio.mu.Unlock()
	if err != nil {
		return err
	}
	if !drop {
		var packet [PacketLen]byte
		copy(packet[:], payload)
		radio.sent <- packet
	}
	return nil
}

func (radio *fakeRadio) Received() <-chan [PacketLen]byte {
	radio.receivedCalls <- struct{}{}
	return radio.recv
}
func (radio *fakeRadio) ReplyGuard() time.Duration { return radio.guard }

func newTestEngine(t *testing.T, timeout time.Duration, retries int) (*Engine, *fakeRadio, protocol.Codec) {
	t.Helper()
	engine := New(Options{HubAddr: hubAddr, Timeout: timeout, Retries: retries})
	radio := newFakeRadio()
	if err := engine.AddRadio(context.Background(), testChannel, radio); err != nil {
		t.Fatal(err)
	}
	codec := addTestNode(t, engine, "node", testChannel, nodeAddr, testKey)
	return engine, radio, codec
}

func addTestNode(t *testing.T, engine *Engine, name string, channel uint8, address [4]byte, key [16]byte) protocol.Codec {
	t.Helper()
	n := node.NewNode(name, channel, testDescriptor(t), node.Identity{Address: address, Key: key})
	if err := engine.AddNode(n); err != nil {
		t.Fatal(err)
	}
	codec, err := protocol.NewCodec(key)
	if err != nil {
		t.Fatal(err)
	}
	return codec
}

func injectResponse(radio *fakeRadio, codec protocol.Codec, source [4]byte, typ, flags byte, reg uint16, value int32) {
	var packet [PacketLen]byte
	codec.Encode(packet[:], source, typ, flags, reg, value)
	radio.recv <- packet
}

func TestGetCarriesGuardAndReturnsValue(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 50*time.Millisecond, 1)
	radio.guard = 20 * time.Millisecond

	go func() {
		request := <-radio.sent
		_, typ, flags, reg, _, err := codec.Decode(request[:])
		if err != nil {
			t.Errorf("decode GET: %v", err)
			return
		}
		if typ != protocol.TypeGET || reg != regTemp || protocol.GuardMillis(flags) != 20 {
			t.Errorf("GET = type %d flags %#x reg %#x", typ, flags, reg)
		}
		injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 4242)
	}()

	update, err := engine.Get(context.Background(), nodeAddr, regTemp)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if update != (Update{Value: 4242}) {
		t.Fatalf("Get = %+v, want value 4242", update)
	}
}

func TestSetAndSetNullCarryGuard(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 50*time.Millisecond, 1)
	radio.guard = 7 * time.Millisecond

	go func() {
		for index := 0; index < 2; index++ {
			request := <-radio.sent
			_, typ, flags, reg, value, err := codec.Decode(request[:])
			if err != nil {
				t.Errorf("decode SET: %v", err)
				return
			}
			if typ != protocol.TypeSET || protocol.GuardMillis(flags) != 7 {
				t.Errorf("SET = type %d flags %#x", typ, flags)
			}
			if index == 0 && (flags&protocol.FlagNULL != 0 || value != 250) {
				t.Errorf("ordinary SET = flags %#x value %d", flags, value)
			}
			if index == 1 && flags&protocol.FlagNULL == 0 {
				t.Errorf("null SET flags = %#x", flags)
			}
			injectResponse(radio, codec, nodeAddr, protocol.TypeACK, flags, reg, 0)
		}
	}()

	if err := engine.Set(context.Background(), nodeAddr, regTemp, 250); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := engine.SetNull(context.Background(), nodeAddr, regTemp); err != nil {
		t.Fatalf("SetNull: %v", err)
	}
}

func TestGetRetriesThenSucceeds(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 20*time.Millisecond, 1)
	radio.mu.Lock()
	radio.drop = 1
	radio.mu.Unlock()
	go func() {
		request := <-radio.sent
		_, typ, flags, reg, _, err := codec.Decode(request[:])
		if err != nil || typ != protocol.TypeGET {
			t.Errorf("retry GET type %d err %v", typ, err)
			return
		}
		injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 7)
	}()

	update, err := engine.Get(context.Background(), nodeAddr, regTemp)
	if err != nil || update.Value != 7 {
		t.Fatalf("Get after retry = %+v, %v", update, err)
	}
}

func TestRetryDuplicateCannotCompleteNextTransaction(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 20*time.Millisecond, 1)
	firstDone := make(chan struct {
		update Update
		err    error
	}, 1)
	go func() {
		update, err := engine.Get(context.Background(), nodeAddr, regTemp)
		firstDone <- struct {
			update Update
			err    error
		}{update, err}
	}()

	firstRequest := <-radio.sent
	_, _, flags, reg, _, _ := codec.Decode(firstRequest[:])
	<-radio.sent
	injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 11)

	secondDone := make(chan struct {
		update Update
		err    error
	}, 1)
	go func() {
		update, err := engine.Get(context.Background(), nodeAddr, regTemp)
		secondDone <- struct {
			update Update
			err    error
		}{update, err}
	}()
	injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 11)

	first := <-firstDone
	if first.err != nil || first.update.Value != 11 {
		t.Fatalf("first Get = %+v, %v", first.update, first.err)
	}
	request := <-radio.sent
	_, _, flags, reg, _, _ = codec.Decode(request[:])
	injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 22)
	second := <-secondDone
	if second.err != nil || second.update.Value != 22 {
		t.Fatalf("second Get = %+v, %v; retry duplicate escaped drain", second.update, second.err)
	}
	if got := engine.SnapshotNode(nodeAddr).Packet.RxOrphanVALUE; got != 1 {
		t.Fatalf("orphan VALUEs = %d, want 1 drained duplicate", got)
	}
}

func TestRetryDuplicateACKCannotCompleteNextSet(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 20*time.Millisecond, 1)
	firstDone := make(chan error, 1)
	go func() { firstDone <- engine.Set(context.Background(), nodeAddr, regTemp, 11) }()

	firstRequest := <-radio.sent
	_, _, flags, reg, _, _ := codec.Decode(firstRequest[:])
	<-radio.sent
	injectResponse(radio, codec, nodeAddr, protocol.TypeACK, flags, reg, 0)

	secondDone := make(chan error, 1)
	go func() { secondDone <- engine.Set(context.Background(), nodeAddr, regTemp, 22) }()
	injectResponse(radio, codec, nodeAddr, protocol.TypeACK, flags, reg, 0)
	if err := <-firstDone; err != nil {
		t.Fatalf("first Set: %v", err)
	}

	request := <-radio.sent
	_, _, flags, reg, _, _ = codec.Decode(request[:])
	injectResponse(radio, codec, nodeAddr, protocol.TypeACK, flags, reg, 0)
	if err := <-secondDone; err != nil {
		t.Fatalf("second Set: %v; retry duplicate escaped drain", err)
	}
	stats := engine.SnapshotNode(nodeAddr)
	if stats.Packet.RxMatchedACK != 2 || stats.Packet.RxOrphanACK != 1 {
		t.Fatalf("ACK stats = %+v", stats.Packet)
	}
}

func TestGetIgnoresWrongSourceRegisterAndType(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 80*time.Millisecond, 1)
	codec2 := addTestNode(t, engine, "other", testChannel, nodeAddr2, testKey2)

	go func() {
		request := <-radio.sent
		_, _, flags, _, _, _ := codec.Decode(request[:])
		injectResponse(radio, codec2, nodeAddr2, protocol.TypeVALUE, flags, regTemp, 1)
		injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, regOther, 2)
		injectResponse(radio, codec, nodeAddr, protocol.TypeACK, flags, regTemp, 0)
		injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, regTemp, 4)
	}()

	update, err := engine.Get(context.Background(), nodeAddr, regTemp)
	if err != nil || update.Value != 4 {
		t.Fatalf("Get = %+v, %v; wrong response completed transaction", update, err)
	}
	stats := engine.SnapshotNode(nodeAddr).Packet
	if stats.RxOrphanVALUE != 1 || stats.RxOrphanACK != 1 || stats.RxMatchedVALUE != 1 {
		t.Fatalf("source-node packet stats = %+v", stats)
	}
	if got := engine.SnapshotNode(nodeAddr2).Packet.RxOrphanVALUE; got != 1 {
		t.Fatalf("wrong-source orphan VALUEs = %d, want 1", got)
	}
}

func TestSameChannelTransactionsQueue(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 200*time.Millisecond, 1)
	firstResult := make(chan Update, 1)
	secondResult := make(chan Update, 1)
	go func() {
		update, _ := engine.Get(context.Background(), nodeAddr, regTemp)
		firstResult <- update
	}()
	firstRequest := <-radio.sent

	go func() {
		update, _ := engine.Get(context.Background(), nodeAddr, regTemp)
		secondResult <- update
	}()
	select {
	case <-radio.sent:
		t.Fatal("second same-channel transaction transmitted before first completed")
	case <-time.After(20 * time.Millisecond):
	}

	_, _, flags, reg, _, _ := codec.Decode(firstRequest[:])
	injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 1)
	secondRequest := <-radio.sent
	_, _, flags, reg, _, _ = codec.Decode(secondRequest[:])
	injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 2)

	if first := <-firstResult; first.Value != 1 {
		t.Fatalf("first result = %+v", first)
	}
	if second := <-secondResult; second.Value != 2 {
		t.Fatalf("second result = %+v", second)
	}
}

func TestDistinctChannelsProceedConcurrently(t *testing.T) {
	engine := New(Options{HubAddr: hubAddr, Timeout: 200 * time.Millisecond, Retries: 1})
	radio1 := newFakeRadio()
	radio2 := newFakeRadio()
	if err := engine.AddRadio(context.Background(), testChannel, radio1); err != nil {
		t.Fatal(err)
	}
	if err := engine.AddRadio(context.Background(), testChannel2, radio2); err != nil {
		t.Fatal(err)
	}
	codec1 := addTestNode(t, engine, "one", testChannel, nodeAddr, testKey)
	codec2 := addTestNode(t, engine, "two", testChannel2, nodeAddr2, testKey2)
	done := make(chan error, 2)
	go func() { _, err := engine.Get(context.Background(), nodeAddr, regTemp); done <- err }()
	go func() { _, err := engine.Get(context.Background(), nodeAddr2, regTemp); done <- err }()

	request1 := <-radio1.sent
	request2 := <-radio2.sent
	_, _, flags1, reg1, _, _ := codec1.Decode(request1[:])
	_, _, flags2, reg2, _, _ := codec2.Decode(request2[:])
	injectResponse(radio1, codec1, nodeAddr, protocol.TypeVALUE, flags1, reg1, 1)
	injectResponse(radio2, codec2, nodeAddr2, protocol.TypeVALUE, flags2, reg2, 2)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

type observedContext struct {
	context.Context
	once    sync.Once
	checked chan struct{}
}

func (ctx *observedContext) Err() error {
	ctx.once.Do(func() { close(ctx.checked) })
	return nil
}

func TestRadioReplacementPreservesChannelGate(t *testing.T) {
	engine, oldRadio, codec := newTestEngine(t, 20*time.Millisecond, 1)
	firstDone := make(chan struct {
		update Update
		err    error
	}, 1)
	go func() {
		update, err := engine.Get(context.Background(), nodeAddr, regTemp)
		firstDone <- struct {
			update Update
			err    error
		}{update, err}
	}()
	<-oldRadio.sent
	<-oldRadio.receivedCalls

	queuedContext := &observedContext{Context: context.Background(), checked: make(chan struct{})}
	secondDone := make(chan struct {
		update Update
		err    error
	}, 1)
	go func() {
		update, err := engine.Get(queuedContext, nodeAddr, regOther)
		secondDone <- struct {
			update Update
			err    error
		}{update, err}
	}()
	<-queuedContext.checked

	newRadio := newFakeRadio()
	if err := engine.AddRadio(context.Background(), testChannel, newRadio); err != nil {
		t.Fatalf("replace radio: %v", err)
	}
	retry := <-oldRadio.sent
	_, _, flags, reg, _, _ := codec.Decode(retry[:])
	injectResponse(oldRadio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 11)

	first := <-firstDone
	if first.err != nil || first.update.Value != 11 {
		t.Fatalf("first Get on old radio = %+v, %v", first.update, first.err)
	}
	request := <-newRadio.sent
	_, _, flags, reg, _, _ = codec.Decode(request[:])
	injectResponse(newRadio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 22)
	second := <-secondDone
	if second.err != nil || second.update.Value != 22 {
		t.Fatalf("queued Get on replacement radio = %+v, %v", second.update, second.err)
	}
	if got := len(newRadio.sent); got != 0 {
		t.Fatalf("replacement radio has %d unexpected extra sends", got)
	}
}

func TestContextCancellationWhileQueued(t *testing.T) {
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
		t.Fatalf("queued Get error = %v, want context canceled", err)
	}
	select {
	case <-radio.sent:
		t.Fatal("canceled queued transaction transmitted")
	default:
	}
	_, _, flags, reg, _, _ := codec.Decode(request[:])
	injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 1)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
}

type stagedCancelContext struct {
	context.Context
	mu       sync.Mutex
	errCalls int
	cancelAt int
}

func (ctx *stagedCancelContext) Err() error {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.errCalls++
	if ctx.errCalls >= ctx.cancelAt {
		return context.Canceled
	}
	return nil
}

func TestCancellationBeforeRetryPreventsSend(t *testing.T) {
	engine, radio, _ := newTestEngine(t, 10*time.Millisecond, 1)
	ctx := &stagedCancelContext{Context: context.Background(), cancelAt: 4}
	if _, err := engine.Get(ctx, nodeAddr, regTemp); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get error = %v, want context canceled", err)
	}
	if got := len(radio.sent); got != 1 {
		t.Fatalf("send count = %d, want no retry after cancellation", got)
	}
}

func TestCancellationAfterSendDrainsBeforeRelease(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 40*time.Millisecond, 0)
	ctx, cancel := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() { _, err := engine.Get(ctx, nodeAddr, regTemp); firstDone <- err }()
	request := <-radio.sent
	<-radio.receivedCalls
	_, _, flags, reg, _, _ := codec.Decode(request[:])
	cancel()
	<-radio.receivedCalls

	secondDone := make(chan struct {
		update Update
		err    error
	}, 1)
	go func() {
		update, err := engine.Get(context.Background(), nodeAddr, regTemp)
		secondDone <- struct {
			update Update
			err    error
		}{update, err}
	}()
	injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 11)
	if err := <-firstDone; !errors.Is(err, context.Canceled) {
		t.Fatalf("first Get error = %v, want context canceled", err)
	}

	request = <-radio.sent
	_, _, flags, reg, _, _ = codec.Decode(request[:])
	injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 22)
	second := <-secondDone
	if second.err != nil || second.update.Value != 22 {
		t.Fatalf("second Get = %+v, %v; canceled transaction reply escaped drain", second.update, second.err)
	}
}

func TestCancellationDuringRetrySuccessDrainDoesNotReleaseGate(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 20*time.Millisecond, 1)
	radio.guard = 8 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	firstDone := make(chan struct {
		update Update
		err    error
	}, 1)
	go func() {
		update, err := engine.Get(ctx, nodeAddr, regTemp)
		firstDone <- struct {
			update Update
			err    error
		}{update, err}
	}()
	request := <-radio.sent
	<-radio.receivedCalls
	_, _, flags, reg, _, _ := codec.Decode(request[:])
	<-radio.sent
	<-radio.receivedCalls
	injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 11)
	<-radio.receivedCalls
	cancel()

	secondDone := make(chan struct {
		update Update
		err    error
	}, 1)
	go func() {
		update, err := engine.Get(context.Background(), nodeAddr, regTemp)
		secondDone <- struct {
			update Update
			err    error
		}{update, err}
	}()
	injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 11)
	select {
	case <-radio.sent:
		t.Fatal("next transaction sent while retry-success drain still owned channel")
	case <-time.After(5 * time.Millisecond):
	}

	first := <-firstDone
	if first.err != nil || first.update.Value != 11 {
		t.Fatalf("first Get = %+v, %v; cancellation aborted successful drain", first.update, first.err)
	}
	request = <-radio.sent
	_, _, flags, reg, _, _ = codec.Decode(request[:])
	injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 22)
	second := <-secondDone
	if second.err != nil || second.update.Value != 22 {
		t.Fatalf("second Get = %+v, %v; duplicate escaped canceled drain", second.update, second.err)
	}
}

func TestFinalTimeoutQuarantinesLateReply(t *testing.T) {
	engine, radio, codec := newTestEngine(t, 20*time.Millisecond, 1)
	firstDone := make(chan error, 1)
	go func() { _, err := engine.Get(context.Background(), nodeAddr, regTemp); firstDone <- err }()
	<-radio.sent
	secondAttempt := <-radio.sent
	_, _, flags, reg, _, _ := codec.Decode(secondAttempt[:])
	time.AfterFunc(22*time.Millisecond, func() {
		injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 11)
	})
	if err := <-firstDone; !errors.Is(err, ErrTimeout) {
		t.Fatalf("first Get error = %v, want timeout", err)
	}

	secondDone := make(chan struct {
		update Update
		err    error
	}, 1)
	go func() {
		update, err := engine.Get(context.Background(), nodeAddr, regTemp)
		secondDone <- struct {
			update Update
			err    error
		}{update, err}
	}()
	request := <-radio.sent
	_, _, flags, reg, _, _ = codec.Decode(request[:])
	injectResponse(radio, codec, nodeAddr, protocol.TypeVALUE, flags, reg, 22)
	result := <-secondDone
	if result.err != nil || result.update.Value != 22 {
		t.Fatalf("second Get = %+v, %v; late reply escaped quarantine", result.update, result.err)
	}
}

func TestAddRadioRejectsLargeGuard(t *testing.T) {
	engine := New(Options{HubAddr: hubAddr, Timeout: 20 * time.Millisecond, Retries: 1})
	radio := newFakeRadio()
	radio.guard = 20 * time.Millisecond
	if err := engine.AddRadio(context.Background(), testChannel, radio); !errors.Is(err, ErrGuardTooLarge) {
		t.Fatalf("AddRadio error = %v, want ErrGuardTooLarge", err)
	}
}

func TestZeroRetriesMeansOneAttempt(t *testing.T) {
	engine, radio, _ := newTestEngine(t, 10*time.Millisecond, 0)
	if _, err := engine.Get(context.Background(), nodeAddr, regTemp); !errors.Is(err, ErrTimeout) {
		t.Fatalf("Get error = %v, want timeout", err)
	}
	if got := len(radio.sent); got != 1 {
		t.Fatalf("send count = %d, want one attempt", got)
	}
}

func TestClosedReceivedChannelWhileWaitingEndsTransaction(t *testing.T) {
	engine, radio, _ := newTestEngine(t, 20*time.Millisecond, 1)
	done := make(chan error, 1)
	go func() {
		_, err := engine.Get(context.Background(), nodeAddr, regTemp)
		done <- err
	}()
	<-radio.sent
	<-radio.receivedCalls
	close(radio.recv)
	<-radio.sent
	select {
	case err := <-done:
		if !errors.Is(err, ErrTimeout) {
			t.Fatalf("Get error = %v, want timeout", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("closed receive channel left transaction spinning")
	}
	get := engine.SnapshotNode(nodeAddr).Transactions[TransactionGet]
	if get.AttemptInitial != 1 || get.AttemptRetry != 1 || get.AttemptTimeout != 2 || get.Outcomes[TransactionTimeout] != 1 {
		t.Fatalf("closed receive transaction stats = %+v", get)
	}
}
