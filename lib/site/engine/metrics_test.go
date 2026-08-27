package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/burgrp/bleriot/lib/shared/protocol"
	"github.com/burgrp/bleriot/lib/site/node"
)

// TestSnapshotNode_CountsGet checks a successful GET's exact transaction,
// packet, latency, and liveness accounting.
func TestSnapshotNode_CountsGet(t *testing.T) {
	e, f, c, cancel := newEngine(t)
	defer cancel()
	simulateNode(t, f, c, func(typ byte, reg uint16, val int32) (int32, bool) {
		return 1, false
	})

	if _, err := e.Get(context.Background(), nodeAddr, regTemp); err != nil {
		t.Fatalf("Get: %v", err)
	}

	s := e.SnapshotNode(nodeAddr)
	get := s.Transactions[TransactionGet]
	if get.Outcomes[TransactionSuccessFirst] != 1 || get.AttemptInitial != 1 || get.AttemptRetry != 0 {
		t.Errorf("GET stats = %+v, want first-attempt success", get)
	}
	if get.LatencyCount != 1 || s.Latency.Count != 1 || s.Latency.Buckets[LatencyBucketCount-1] != 1 {
		t.Errorf("latency stats = %+v / %+v, want one observation", get, s.Latency)
	}
	if s.Packet.RxValid != 1 || s.Packet.RxSolicitedIS != 1 || s.Packet.TxSuccess != 1 {
		t.Errorf("packet stats = %+v, want one successful TX and solicited IS", s.Packet)
	}
	if s.Liveness.State != LivenessOnline || s.Liveness.TransitionsOnline != 1 {
		t.Errorf("liveness = %+v, want first transition to online", s.Liveness)
	}
}

// TestSnapshotNode_CountsSetAck checks a SET counts an ACK, not an IS.
func TestSnapshotNode_CountsSetAck(t *testing.T) {
	e, f, c, cancel := newEngine(t)
	defer cancel()
	simulateNode(t, f, c, func(typ byte, reg uint16, val int32) (int32, bool) {
		return val, false
	})

	if err := e.Set(context.Background(), nodeAddr, regTemp, 5); err != nil {
		t.Fatalf("Set: %v", err)
	}

	s := e.SnapshotNode(nodeAddr)
	if s.Transactions[TransactionSet].Outcomes[TransactionSuccessFirst] != 1 {
		t.Errorf("SET stats = %+v, want first-attempt success", s.Transactions[TransactionSet])
	}
	if s.Packet.RxMatchedACK != 1 {
		t.Errorf("packet stats = %+v, want one matched ACK", s.Packet)
	}
}

// TestSnapshotNode_Timeout checks an unanswered transaction counts every send
// (initial plus retries) and one timeout, and leaves the node offline.
func TestSnapshotNode_Timeout(t *testing.T) {
	e, _, _, cancel := newEngine(t)
	defer cancel()

	if _, err := e.Get(context.Background(), nodeAddr, regTemp); err != ErrTimeout {
		t.Fatalf("Get err = %v, want ErrTimeout", err)
	}

	s := e.SnapshotNode(nodeAddr)
	get := s.Transactions[TransactionGet]
	if get.AttemptInitial != 1 || get.AttemptRetry != 3 || get.ResponseTimeout != 4 {
		t.Errorf("GET attempts = %+v, want 1 initial, 3 retry, 4 response timeouts", get)
	}
	if get.Outcomes[TransactionTimeout] != 1 {
		t.Errorf("GET timeout outcomes = %d, want 1", get.Outcomes[TransactionTimeout])
	}
	if s.Packet.TxSuccess != 4 || s.Packet.TxError != 0 {
		t.Errorf("packet TX stats = %+v, want four successful sends", s.Packet)
	}
	if s.Liveness.State != LivenessUnknown {
		t.Errorf("liveness = %v, want unknown without probe evidence", s.Liveness.State)
	}
}

// TestSnapshotNode_Corrupt checks an undecodable packet from a known source is
// counted as activity (RxAll) and as corrupt, but not as IS or ACK.
func TestSnapshotNode_Corrupt(t *testing.T) {
	e, f, _, cancel := newEngine(t)
	defer cancel()

	// Inject a packet whose cleartext source is the node but whose ciphertext is
	// garbage, so decode fails and it is attributed as a corrupt receipt.
	var pkt [PacketLen]byte
	copy(pkt[:], nodeAddr[:])
	for i := node.AddrLen; i < PacketLen; i++ {
		pkt[i] = 0xFF
	}
	f.recv <- pkt

	// Wait for the receive loop to process it.
	deadline := time.After(time.Second)
	for {
		if e.SnapshotNode(nodeAddr).Packet.RxTotal >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("corrupt packet was not counted")
		case <-time.After(2 * time.Millisecond):
		}
	}

	s := e.SnapshotNode(nodeAddr)
	if s.Packet.RxTotal != 1 || s.Packet.RxInvalidDecode != 1 || s.Packet.RxValid != 0 || s.Packet.LastValid != 0 {
		t.Errorf("packet stats = %+v, want one decode failure and no valid activity", s.Packet)
	}
}

// TestSnapshotNode_Unknown checks an unknown address yields a zero snapshot.
func TestSnapshotNode_Unknown(t *testing.T) {
	e, _, _, cancel := newEngine(t)
	defer cancel()
	s := e.SnapshotNode([4]byte{9, 9, 9, 9})
	if s != (NodeStats{}) {
		t.Errorf("unknown node snapshot = %+v, want zero", s)
	}
}

func TestSnapshotNode_RetrySuccessOutcome(t *testing.T) {
	e, radio, codec, cancel := newEngine(t)
	defer cancel()
	radio.mu.Lock()
	radio.drop = 2
	radio.mu.Unlock()
	simulateNode(t, radio, codec, func(byte, uint16, int32) (int32, bool) { return 1, false })

	if _, err := e.Get(context.Background(), nodeAddr, regTemp); err != nil {
		t.Fatal(err)
	}
	get := e.SnapshotNode(nodeAddr).Transactions[TransactionGet]
	if get.Outcomes[TransactionSuccessRetry] != 1 || get.AttemptInitial != 1 || get.AttemptRetry != 2 || outcomeTotal(get) != 1 {
		t.Fatalf("retry success stats = %+v, want one retry terminal outcome", get)
	}
}

func TestSnapshotNode_SendErrorOutcome(t *testing.T) {
	e, radio, _, cancel := newEngine(t)
	defer cancel()
	radio.mu.Lock()
	radio.sendErr = errors.New("send failed")
	radio.mu.Unlock()

	if _, err := e.Get(context.Background(), nodeAddr, regTemp); err == nil {
		t.Fatal("Get returned nil error")
	}
	get := e.SnapshotNode(nodeAddr).Transactions[TransactionGet]
	if get.Outcomes[TransactionSendError] != 1 || get.AttemptSendError != 1 || outcomeTotal(get) != 1 {
		t.Fatalf("send error stats = %+v, want one send-error outcome", get)
	}
}

func TestSnapshotNode_CanceledOutcome(t *testing.T) {
	e, _, _, cancelEngine := newEngine(t)
	defer cancelEngine()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := e.Get(ctx, nodeAddr, regTemp); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get error = %v, want context canceled", err)
	}
	get := e.SnapshotNode(nodeAddr).Transactions[TransactionGet]
	if get.Outcomes[TransactionCanceled] != 1 || outcomeTotal(get) != 1 {
		t.Fatalf("canceled stats = %+v, want one canceled outcome", get)
	}
}

func TestSnapshotNode_BusyOutcome(t *testing.T) {
	e, radio, _, cancelEngine := newEngine(t)
	defer cancelEngine()
	ctx, cancel := context.WithCancel(context.Background())
	firstDone := make(chan error, 1)
	go func() {
		_, err := e.Get(ctx, nodeAddr, regTemp)
		firstDone <- err
	}()
	<-radio.sent

	if _, err := e.Get(context.Background(), nodeAddr, regTemp); !errors.Is(err, ErrBusy) {
		t.Fatalf("second Get error = %v, want busy", err)
	}
	cancel()
	<-firstDone
	get := e.SnapshotNode(nodeAddr).Transactions[TransactionGet]
	if get.Outcomes[TransactionBusy] != 1 || get.Outcomes[TransactionCanceled] != 1 || outcomeTotal(get) != 2 {
		t.Fatalf("busy stats = %+v, want one busy and one canceled outcome", get)
	}
}

func TestSnapshotNode_NoRadioOutcome(t *testing.T) {
	e, _, _, cancel := newEngine(t)
	defer cancel()
	e.mu.Lock()
	delete(e.radios, testChannel)
	e.mu.Unlock()

	if _, err := e.Get(context.Background(), nodeAddr, regTemp); !errors.Is(err, ErrNoRadio) {
		t.Fatalf("Get error = %v, want no radio", err)
	}
	get := e.SnapshotNode(nodeAddr).Transactions[TransactionGet]
	if get.Outcomes[TransactionNoRadio] != 1 || outcomeTotal(get) != 1 {
		t.Fatalf("no-radio stats = %+v, want one no-radio outcome", get)
	}
}

func TestSnapshotNode_WatchAllLiveness(t *testing.T) {
	e, _, _, cancel := newEngine(t)
	defer cancel()
	e.mu.Lock()
	e.watchAll[nodeAddr] = func(uint16, Update) {}
	e.mu.Unlock()

	e.noteLivenessAll(nodeAddr, ErrTimeout)
	stats := e.SnapshotNode(nodeAddr).Liveness
	if stats.State != LivenessSuspect || stats.Misses != 1 || stats.TransitionsSuspect != 1 {
		t.Fatalf("first missed watch-all = %+v, want suspect with one miss", stats)
	}

	e.noteLivenessAll(nodeAddr, ErrTimeout)
	stats = e.SnapshotNode(nodeAddr).Liveness
	if stats.State != LivenessOffline || stats.Misses != 2 || stats.TransitionsOffline != 1 {
		t.Fatalf("second missed watch-all = %+v, want offline with two misses", stats)
	}
}

func TestSnapshotNode_UnknownRegisterPushIsAcknowledged(t *testing.T) {
	e, radio, codec, cancel := newEngine(t)
	defer cancel()
	const unknownRegister = uint16(0x4321)
	var push [PacketLen]byte
	codec.Encode(push[:], nodeAddr, protocol.TypeIS, protocol.FlagPush, unknownRegister, 7)
	radio.recv <- push

	select {
	case rawACK := <-radio.sent:
		_, packetType, _, register, _, err := codec.Decode(rawACK[:])
		if err != nil || packetType != protocol.TypeACK || register != unknownRegister {
			t.Fatalf("push ACK = type %d register %04x err %v", packetType, register, err)
		}
	case <-time.After(time.Second):
		t.Fatal("unknown-register push was not acknowledged")
	}

	packet := e.SnapshotNode(nodeAddr).Packet
	if packet.RxUnknownRegister != 1 || packet.RxPushIS != 1 || packet.PushACKSuccess != 1 {
		t.Fatalf("unknown-register packet stats = %+v", packet)
	}
}

func outcomeTotal(stats TransactionStats) uint64 {
	var total uint64
	for _, count := range stats.Outcomes {
		total += count
	}
	return total
}
