//go:build dongles

package functest

import (
	"context"
	"testing"
)

// These benchmarks measure end-to-end transaction latency over the real RF link
// between two MCP2210 dongles (hub engine ↔ node runtime). Each iteration is one
// full protocol transaction — request packet out, node decode + device access,
// reply packet back, hub decode — so the reported ns/op is the wall-clock round
// trip dominated by USB-HID and over-the-air time.
//
// Run with the dongles env vars set (see the Makefile `bench` target):
//
//	go test -tags dongles -bench . -benchmem -run '^$' ./functest/...
//
// Report ms/op as well as ns/op since these are millisecond-scale operations.

func BenchmarkGet(b *testing.B) {
	h := setup(b)
	h.seed(tagTemp, 2137)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u, err := h.eng.Get(ctx, nodeAddr, tagTemp)
		if err != nil {
			b.Fatalf("Get: %v", err)
		}
		if u.Null {
			b.Fatalf("Get returned null")
		}
	}
	b.StopTimer()
	reportLatency(b)
}

func BenchmarkSet(b *testing.B) {
	h := setup(b)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := h.eng.Set(ctx, nodeAddr, tagSetting, int32(i)); err != nil {
			b.Fatalf("Set: %v", err)
		}
	}
	b.StopTimer()
	reportLatency(b)
}

// reportLatency adds a human-friendly ms/op metric alongside the default ns/op.
func reportLatency(b *testing.B) {
	if b.N > 0 {
		nsPerOp := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		b.ReportMetric(nsPerOp/1e6, "ms/op")
	}
}
