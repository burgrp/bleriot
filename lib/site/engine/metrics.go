package engine

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/burgrp/bleriot/lib/site/node"
)

// TransactionOperation identifies why the engine started a transaction.
type TransactionOperation uint8

const (
	TransactionGet TransactionOperation = iota
	TransactionSet
	TransactionWatch
	TransactionUnwatch
	TransactionRefresh
	TransactionOperationCount
)

func (operation TransactionOperation) String() string {
	switch operation {
	case TransactionGet:
		return "get"
	case TransactionSet:
		return "set"
	case TransactionWatch:
		return "watch"
	case TransactionUnwatch:
		return "unwatch"
	case TransactionRefresh:
		return "refresh"
	default:
		return "unknown"
	}
}

// TransactionOutcome is the single terminal result of a transaction.
type TransactionOutcome uint8

const (
	TransactionSuccessFirst TransactionOutcome = iota
	TransactionSuccessRetry
	TransactionTimeout
	TransactionSendError
	TransactionCanceled
	TransactionBusy
	TransactionNoRadio
	TransactionOutcomeCount
)

func (outcome TransactionOutcome) String() string {
	switch outcome {
	case TransactionSuccessFirst:
		return "success_first"
	case TransactionSuccessRetry:
		return "success_retry"
	case TransactionTimeout:
		return "timeout"
	case TransactionSendError:
		return "send_error"
	case TransactionCanceled:
		return "canceled"
	case TransactionBusy:
		return "busy"
	case TransactionNoRadio:
		return "no_radio"
	default:
		return "unknown"
	}
}

// LatencyBucketCount includes seven finite buckets and a cumulative +Inf bucket.
const LatencyBucketCount = 8

var latencyUpperBounds = [...]time.Duration{
	25 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	200 * time.Millisecond,
	500 * time.Millisecond,
	time.Second,
	2 * time.Second,
}

// LatencyBucketLabel returns the bucket upper bound in seconds.
func LatencyBucketLabel(index int) string {
	if index == LatencyBucketCount-1 {
		return "+Inf"
	}
	if index < 0 || index >= len(latencyUpperBounds) {
		return ""
	}
	return strconv.FormatFloat(latencyUpperBounds[index].Seconds(), 'f', -1, 64)
}

type transactionMetrics struct {
	outcomes         [TransactionOutcomeCount]atomic.Uint64
	attemptInitial   atomic.Uint64
	attemptRetry     atomic.Uint64
	attemptSendError atomic.Uint64
	responseTimeout  atomic.Uint64
	latencyCount     atomic.Uint64
	latencySumMicros atomic.Uint64
}

type packetMetrics struct {
	rxTotal           atomic.Uint64
	rxValid           atomic.Uint64
	rxPushIS          atomic.Uint64
	rxSolicitedIS     atomic.Uint64
	rxOrphanIS        atomic.Uint64
	rxMatchedACK      atomic.Uint64
	rxOrphanACK       atomic.Uint64
	rxNullIS          atomic.Uint64
	rxInvalidDecode   atomic.Uint64
	rxInvalidSource   atomic.Uint64
	rxInvalidType     atomic.Uint64
	rxUnknownRegister atomic.Uint64
	txSuccess         atomic.Uint64
	txError           atomic.Uint64
	pushACKSuccess    atomic.Uint64
	pushACKError      atomic.Uint64
	pushACKNoRadio    atomic.Uint64
	lastReceived      atomic.Int64
	lastValid         atomic.Int64
}

// LivenessState is the engine's current evidence-based view of a node.
type LivenessState uint32

const (
	LivenessUnknown LivenessState = iota
	LivenessOnline
	LivenessSuspect
	LivenessOffline
)

func (state LivenessState) String() string {
	switch state {
	case LivenessOnline:
		return "online"
	case LivenessSuspect:
		return "suspect"
	case LivenessOffline:
		return "offline"
	default:
		return "unknown"
	}
}

type livenessMetrics struct {
	state              atomic.Uint32
	since              atomic.Int64
	lastSuccess        atomic.Int64
	lastFailure        atomic.Int64
	transitionsOnline  atomic.Uint64
	transitionsSuspect atomic.Uint64
	transitionsOffline atomic.Uint64
}

// nodeMetrics holds the cumulative diagnostic counters for one node. All fields
// are updated lock-free from the transact (TX) and handle (RX) paths and read by
// SnapshotNode; the *nodeMetrics pointer itself is created once in AddNode and
// never moves, so concurrent counter updates are safe without the engine lock.
type nodeMetrics struct {
	transactions     [TransactionOperationCount]transactionMetrics
	latencyCount     atomic.Uint64
	latencySumMicros atomic.Uint64
	latencyBuckets   [LatencyBucketCount]atomic.Uint64
	packet           packetMetrics
	liveness         livenessMetrics
}

// TransactionStats is a cumulative process-lifetime transaction snapshot.
type TransactionStats struct {
	Outcomes         [TransactionOutcomeCount]uint64
	AttemptInitial   uint64
	AttemptRetry     uint64
	AttemptSendError uint64
	ResponseTimeout  uint64
	LatencyCount     uint64
	LatencySumMicros uint64
}

// LatencyStats is a cumulative histogram of successful transaction wall time.
type LatencyStats struct {
	Count     uint64
	SumMicros uint64
	Buckets   [LatencyBucketCount]uint64
}

// PacketStats separates raw reception, validation, semantic packet classes,
// and actual radio send outcomes.
type PacketStats struct {
	RxTotal           uint64
	RxValid           uint64
	RxPushIS          uint64
	RxSolicitedIS     uint64
	RxOrphanIS        uint64
	RxMatchedACK      uint64
	RxOrphanACK       uint64
	RxNullIS          uint64
	RxInvalidDecode   uint64
	RxInvalidSource   uint64
	RxInvalidType     uint64
	RxUnknownRegister uint64
	TxSuccess         uint64
	TxError           uint64
	PushACKSuccess    uint64
	PushACKError      uint64
	PushACKNoRadio    uint64
	LastReceived      int64
	LastValid         int64
}

// LivenessStats captures current state, evidence timestamps, and transitions.
type LivenessStats struct {
	State              LivenessState
	Since              int64
	LastSuccess        int64
	LastFailure        int64
	Misses             int
	TransitionsOnline  uint64
	TransitionsSuspect uint64
	TransitionsOffline uint64
}

// NodeStats is a point-in-time snapshot of a node's diagnostic counters together
// with its derived liveness, as published by the diagnostics bridge.
type NodeStats struct {
	Transactions [TransactionOperationCount]TransactionStats
	Latency      LatencyStats
	Packet       PacketStats
	Liveness     LivenessStats
}

func (metrics *nodeMetrics) recordOutcome(operation TransactionOperation, outcome TransactionOutcome) {
	metrics.transactions[operation].outcomes[outcome].Add(1)
}

func (metrics *nodeMetrics) recordSuccessLatency(operation TransactionOperation, elapsed time.Duration) {
	micros := uint64(max(elapsed.Microseconds(), 0))
	transaction := &metrics.transactions[operation]
	transaction.latencyCount.Add(1)
	transaction.latencySumMicros.Add(micros)
	metrics.latencyCount.Add(1)
	metrics.latencySumMicros.Add(micros)
	for index, upper := range latencyUpperBounds {
		if elapsed <= upper {
			metrics.latencyBuckets[index].Add(1)
		}
	}
	metrics.latencyBuckets[LatencyBucketCount-1].Add(1)
}

func (metrics *nodeMetrics) setLiveness(state LivenessState, now time.Time) {
	previous := LivenessState(metrics.liveness.state.Swap(uint32(state)))
	if previous == state {
		return
	}
	metrics.liveness.since.Store(now.Unix())
	switch state {
	case LivenessOnline:
		metrics.liveness.transitionsOnline.Add(1)
	case LivenessSuspect:
		metrics.liveness.transitionsSuspect.Add(1)
	case LivenessOffline:
		metrics.liveness.transitionsOffline.Add(1)
	}
}

func (metrics *nodeMetrics) livenessSuccess(now time.Time) {
	metrics.liveness.lastSuccess.Store(now.Unix())
	metrics.setLiveness(LivenessOnline, now)
}

func (metrics *nodeMetrics) livenessFailure(misses, threshold int, now time.Time) {
	metrics.liveness.lastFailure.Store(now.Unix())
	if misses >= threshold {
		metrics.setLiveness(LivenessOffline, now)
		return
	}
	metrics.setLiveness(LivenessSuspect, now)
}

// metricsFor returns the metrics for an address, or nil when the address is not a
// known node. The map is populated in AddNode and only read afterwards, so this
// needs no lock beyond the one the callers already hold while resolving a node.
func (e *Engine) metricsFor(addr [node.AddrLen]byte) *nodeMetrics {
	return e.metrics[addr]
}

// SnapshotNode returns a point-in-time view of a node's diagnostics. An unknown
// address yields a zero NodeStats.
func (e *Engine) SnapshotNode(addr [node.AddrLen]byte) NodeStats {
	e.mu.Lock()
	nm := e.metrics[addr]
	if nm == nil {
		e.mu.Unlock()
		return NodeStats{}
	}
	maxMiss := 0
	for k := range e.subs {
		if k.addr != addr {
			continue
		}
		if m := e.misses[k]; m > maxMiss {
			maxMiss = m
		}
	}
	if _, watched := e.watchAll[addr]; watched && e.allMisses[addr] > maxMiss {
		maxMiss = e.allMisses[addr]
	}
	e.mu.Unlock()

	stats := NodeStats{}
	for operation := TransactionOperation(0); operation < TransactionOperationCount; operation++ {
		source := &nm.transactions[operation]
		target := &stats.Transactions[operation]
		for outcome := TransactionOutcome(0); outcome < TransactionOutcomeCount; outcome++ {
			target.Outcomes[outcome] = source.outcomes[outcome].Load()
		}
		target.AttemptInitial = source.attemptInitial.Load()
		target.AttemptRetry = source.attemptRetry.Load()
		target.AttemptSendError = source.attemptSendError.Load()
		target.ResponseTimeout = source.responseTimeout.Load()
		target.LatencyCount = source.latencyCount.Load()
		target.LatencySumMicros = source.latencySumMicros.Load()
	}
	stats.Latency.Count = nm.latencyCount.Load()
	stats.Latency.SumMicros = nm.latencySumMicros.Load()
	for index := range stats.Latency.Buckets {
		stats.Latency.Buckets[index] = nm.latencyBuckets[index].Load()
	}
	stats.Packet = PacketStats{
		RxTotal: nm.packet.rxTotal.Load(), RxValid: nm.packet.rxValid.Load(),
		RxPushIS: nm.packet.rxPushIS.Load(), RxSolicitedIS: nm.packet.rxSolicitedIS.Load(),
		RxOrphanIS: nm.packet.rxOrphanIS.Load(), RxMatchedACK: nm.packet.rxMatchedACK.Load(),
		RxOrphanACK: nm.packet.rxOrphanACK.Load(), RxNullIS: nm.packet.rxNullIS.Load(),
		RxInvalidDecode: nm.packet.rxInvalidDecode.Load(), RxInvalidSource: nm.packet.rxInvalidSource.Load(),
		RxInvalidType: nm.packet.rxInvalidType.Load(), RxUnknownRegister: nm.packet.rxUnknownRegister.Load(),
		TxSuccess: nm.packet.txSuccess.Load(), TxError: nm.packet.txError.Load(),
		PushACKSuccess: nm.packet.pushACKSuccess.Load(), PushACKError: nm.packet.pushACKError.Load(),
		PushACKNoRadio: nm.packet.pushACKNoRadio.Load(), LastReceived: nm.packet.lastReceived.Load(),
		LastValid: nm.packet.lastValid.Load(),
	}
	stats.Liveness = LivenessStats{
		State: LivenessState(nm.liveness.state.Load()), Since: nm.liveness.since.Load(),
		LastSuccess: nm.liveness.lastSuccess.Load(), LastFailure: nm.liveness.lastFailure.Load(),
		Misses: maxMiss, TransitionsOnline: nm.liveness.transitionsOnline.Load(),
		TransitionsSuspect: nm.liveness.transitionsSuspect.Load(),
		TransitionsOffline: nm.liveness.transitionsOffline.Load(),
	}
	return stats
}
