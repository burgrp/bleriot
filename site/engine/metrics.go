package engine

import (
	"sync/atomic"

	"github.com/burgrp/bleriot/site/node"
)

// nodeMetrics holds the cumulative diagnostic counters for one node. All fields
// are updated lock-free from the transact (TX) and handle (RX) paths and read by
// SnapshotNode; the *nodeMetrics pointer itself is created once in AddNode and
// never moves, so concurrent counter updates are safe without the engine lock.
type nodeMetrics struct {
	rxAll     atomic.Uint64 // every received packet attributed to this node
	rxIS      atomic.Uint64 // received IS reports
	rxACK     atomic.Uint64 // received SET acknowledgements
	rxCorrupt atomic.Uint64 // received packets that failed to decode or were bogus
	txAll     atomic.Uint64 // every transmitted packet (initial sends and retries)
	txRetries atomic.Uint64 // retransmissions only (a subset of txAll)
	timeouts  atomic.Uint64 // transactions that exhausted all retries with no reply
	lastRx    atomic.Int64  // unix seconds of the most recent received packet, 0 if none
}

// NodeStats is a point-in-time snapshot of a node's diagnostic counters together
// with its derived liveness, as published by the diagnostics bridge.
type NodeStats struct {
	RxAll     uint64
	RxIS      uint64
	RxACK     uint64
	RxCorrupt uint64
	TxAll     uint64
	TxRetries uint64
	Timeouts  uint64
	// LastRx is the unix-seconds timestamp of the most recent received packet, or
	// 0 if nothing has ever been heard from the node.
	LastRx int64
	// Misses is the current maximum consecutive unanswered WATCH refreshes across
	// the node's active subscriptions (0 when all are answering).
	Misses int
	// Online reports whether the node is currently answering: its Misses are below
	// the engine's liveness threshold.
	Online bool
}

// metricsFor returns the metrics for an address, or nil when the address is not a
// known node. The map is populated in AddNode and only read afterwards, so this
// needs no lock beyond the one the callers already hold while resolving a node.
func (e *Engine) metricsFor(addr [node.AddrLen]byte) *nodeMetrics {
	return e.metrics[addr]
}

// SnapshotNode returns a consistent point-in-time view of a node's diagnostic
// counters and derived liveness. An unknown address yields a zero NodeStats with
// Online false.
func (e *Engine) SnapshotNode(addr [node.AddrLen]byte) NodeStats {
	e.mu.Lock()
	nm := e.metrics[addr]
	if nm == nil {
		e.mu.Unlock()
		return NodeStats{}
	}
	// Derive liveness from the node's subscriptions: the largest current miss
	// count across them, and whether that stays below the liveness threshold.
	maxMiss := 0
	hasSub := false
	for k := range e.subs {
		if k.addr != addr {
			continue
		}
		hasSub = true
		if m := e.misses[k]; m > maxMiss {
			maxMiss = m
		}
	}
	liveness := e.liveness
	e.mu.Unlock()

	online := maxMiss < liveness
	if !hasSub {
		// With no active subscriptions there is no liveness signal; fall back to
		// "have we ever heard from it" so a never-seen node does not read online.
		online = nm.lastRx.Load() != 0
	}
	return NodeStats{
		RxAll:     nm.rxAll.Load(),
		RxIS:      nm.rxIS.Load(),
		RxACK:     nm.rxACK.Load(),
		RxCorrupt: nm.rxCorrupt.Load(),
		TxAll:     nm.txAll.Load(),
		TxRetries: nm.txRetries.Load(),
		Timeouts:  nm.timeouts.Load(),
		LastRx:    nm.lastRx.Load(),
		Misses:    maxMiss,
		Online:    online,
	}
}
