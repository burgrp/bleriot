package engine

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/burgrp/bleriot/lib/site/node"
)

// TransactionOperation identifies the request that started a transaction.
type TransactionOperation uint8

const (
	TransactionGet TransactionOperation = iota
	TransactionSet
	TransactionOperationCount
)

func (operation TransactionOperation) String() string {
	switch operation {
	case TransactionGet:
		return "get"
	case TransactionSet:
		return "set"
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
	attemptTimeout   atomic.Uint64
	latencyCount     atomic.Uint64
	latencySumMicros atomic.Uint64
}

type packetMetrics struct {
	rxTotal           atomic.Uint64
	rxValid           atomic.Uint64
	rxMatchedVALUE    atomic.Uint64
	rxOrphanVALUE     atomic.Uint64
	rxMatchedACK      atomic.Uint64
	rxOrphanACK       atomic.Uint64
	rxNullVALUE       atomic.Uint64
	rxInvalidDecode   atomic.Uint64
	rxInvalidType     atomic.Uint64
	rxUnknownRegister atomic.Uint64
	txSuccess         atomic.Uint64
	txError           atomic.Uint64
	lastReceived      atomic.Int64
	lastValid         atomic.Int64
}

type nodeMetrics struct {
	transactions     [TransactionOperationCount]transactionMetrics
	latencyCount     atomic.Uint64
	latencySumMicros atomic.Uint64
	latencyBuckets   [LatencyBucketCount]atomic.Uint64
	packet           packetMetrics
}

// TransactionStats is a cumulative process-lifetime transaction snapshot.
type TransactionStats struct {
	Outcomes         [TransactionOutcomeCount]uint64
	AttemptInitial   uint64
	AttemptRetry     uint64
	AttemptSendError uint64
	AttemptTimeout   uint64
	LatencyCount     uint64
	LatencySumMicros uint64
}

// LatencyStats is a cumulative histogram of successful transaction wall time,
// including time spent queued for the channel.
type LatencyStats struct {
	Count     uint64
	SumMicros uint64
	Buckets   [LatencyBucketCount]uint64
}

// PacketStats separates raw reception, validation, response classes, and send
// outcomes.
type PacketStats struct {
	RxTotal           uint64
	RxValid           uint64
	RxMatchedVALUE    uint64
	RxOrphanVALUE     uint64
	RxMatchedACK      uint64
	RxOrphanACK       uint64
	RxNullVALUE       uint64
	RxInvalidDecode   uint64
	RxInvalidType     uint64
	RxUnknownRegister uint64
	TxSuccess         uint64
	TxError           uint64
	LastReceived      int64
	LastValid         int64
}

// NodeStats is a point-in-time snapshot of a node's diagnostic counters.
type NodeStats struct {
	Transactions [TransactionOperationCount]TransactionStats
	Latency      LatencyStats
	Packet       PacketStats
}

func (metrics *nodeMetrics) recordOutcome(operation TransactionOperation, outcome TransactionOutcome) {
	metrics.transactions[operation].outcomes[outcome].Add(1)
}

func (metrics *nodeMetrics) recordSuccessLatency(operation TransactionOperation, elapsed time.Duration) {
	micros := elapsed.Microseconds()
	if micros < 0 {
		micros = 0
	}
	transaction := &metrics.transactions[operation]
	transaction.latencyCount.Add(1)
	transaction.latencySumMicros.Add(uint64(micros))
	metrics.latencyCount.Add(1)
	metrics.latencySumMicros.Add(uint64(micros))
	for index, upper := range latencyUpperBounds {
		if elapsed <= upper {
			metrics.latencyBuckets[index].Add(1)
		}
	}
	metrics.latencyBuckets[LatencyBucketCount-1].Add(1)
}

// metricsFor is called while e.mu is held or after a node's metrics pointer has
// been resolved under that lock.
func (e *Engine) metricsFor(addr [node.AddrLen]byte) *nodeMetrics {
	return e.metrics[addr]
}

// SnapshotNode returns a point-in-time view of a node's diagnostics. An unknown
// address yields a zero NodeStats.
func (e *Engine) SnapshotNode(addr [node.AddrLen]byte) NodeStats {
	e.mu.Lock()
	metrics := e.metrics[addr]
	e.mu.Unlock()
	if metrics == nil {
		return NodeStats{}
	}

	stats := NodeStats{}
	for operation := TransactionOperation(0); operation < TransactionOperationCount; operation++ {
		source := &metrics.transactions[operation]
		target := &stats.Transactions[operation]
		for outcome := TransactionOutcome(0); outcome < TransactionOutcomeCount; outcome++ {
			target.Outcomes[outcome] = source.outcomes[outcome].Load()
		}
		target.AttemptInitial = source.attemptInitial.Load()
		target.AttemptRetry = source.attemptRetry.Load()
		target.AttemptSendError = source.attemptSendError.Load()
		target.AttemptTimeout = source.attemptTimeout.Load()
		target.LatencyCount = source.latencyCount.Load()
		target.LatencySumMicros = source.latencySumMicros.Load()
	}
	stats.Latency.Count = metrics.latencyCount.Load()
	stats.Latency.SumMicros = metrics.latencySumMicros.Load()
	for index := range stats.Latency.Buckets {
		stats.Latency.Buckets[index] = metrics.latencyBuckets[index].Load()
	}
	stats.Packet = PacketStats{
		RxTotal: metrics.packet.rxTotal.Load(), RxValid: metrics.packet.rxValid.Load(),
		RxMatchedVALUE: metrics.packet.rxMatchedVALUE.Load(), RxOrphanVALUE: metrics.packet.rxOrphanVALUE.Load(),
		RxMatchedACK: metrics.packet.rxMatchedACK.Load(), RxOrphanACK: metrics.packet.rxOrphanACK.Load(),
		RxNullVALUE: metrics.packet.rxNullVALUE.Load(), RxInvalidDecode: metrics.packet.rxInvalidDecode.Load(),
		RxInvalidType:     metrics.packet.rxInvalidType.Load(),
		RxUnknownRegister: metrics.packet.rxUnknownRegister.Load(), TxSuccess: metrics.packet.txSuccess.Load(),
		TxError: metrics.packet.txError.Load(), LastReceived: metrics.packet.lastReceived.Load(),
		LastValid: metrics.packet.lastValid.Load(),
	}
	return stats
}
