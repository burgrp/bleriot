package engine

import (
	"context"
	"testing"
	"time"

	"github.com/burgrp/bleriot/lib/site/node"
)

// TestSnapshotNode_CountsGet checks a successful GET increments the node's TX and
// RX-IS counters, stamps the last-seen time, and reports the node online.
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
	if s.TxAll != 1 {
		t.Errorf("TxAll = %d, want 1", s.TxAll)
	}
	if s.TxRetries != 0 {
		t.Errorf("TxRetries = %d, want 0", s.TxRetries)
	}
	if s.RxAll != 1 || s.RxIS != 1 {
		t.Errorf("RxAll/RxIS = %d/%d, want 1/1", s.RxAll, s.RxIS)
	}
	if s.RxACK != 0 || s.RxCorrupt != 0 {
		t.Errorf("RxACK/RxCorrupt = %d/%d, want 0/0", s.RxACK, s.RxCorrupt)
	}
	if s.Timeouts != 0 {
		t.Errorf("Timeouts = %d, want 0", s.Timeouts)
	}
	if s.LastRx == 0 {
		t.Error("LastRx = 0, want a recent timestamp")
	}
	if !s.Online {
		t.Error("Online = false, want true after a reply")
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
	if s.RxACK != 1 {
		t.Errorf("RxACK = %d, want 1", s.RxACK)
	}
	if s.RxIS != 0 {
		t.Errorf("RxIS = %d, want 0", s.RxIS)
	}
	if s.RxAll != 1 {
		t.Errorf("RxAll = %d, want 1", s.RxAll)
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
	// Retries=3 in newEngine: one initial send plus three retries.
	if s.TxAll != 4 {
		t.Errorf("TxAll = %d, want 4", s.TxAll)
	}
	if s.TxRetries != 3 {
		t.Errorf("TxRetries = %d, want 3", s.TxRetries)
	}
	if s.Timeouts != 1 {
		t.Errorf("Timeouts = %d, want 1", s.Timeouts)
	}
	if s.RxAll != 0 {
		t.Errorf("RxAll = %d, want 0", s.RxAll)
	}
	if s.Online {
		t.Error("Online = true, want false (never heard from, no subs)")
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
		if e.SnapshotNode(nodeAddr).RxAll >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("corrupt packet was not counted")
		case <-time.After(2 * time.Millisecond):
		}
	}

	s := e.SnapshotNode(nodeAddr)
	if s.RxCorrupt != 1 {
		t.Errorf("RxCorrupt = %d, want 1", s.RxCorrupt)
	}
	if s.RxIS != 0 || s.RxACK != 0 {
		t.Errorf("RxIS/RxACK = %d/%d, want 0/0", s.RxIS, s.RxACK)
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
