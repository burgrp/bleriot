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

// forEachSpreadBench runs fn once per spreading factor in spreadConfigs, each as
// a named sub-benchmark ("S8", "S2") with a freshly set-up harness on that
// factor's channel, so latency is reported for both BLE Coded PHY factors.
func forEachSpreadBench(b *testing.B, fn func(b *testing.B, h *harness)) {
	b.Helper()
	for _, sc := range spreadConfigs {
		b.Run(sc.name, func(b *testing.B) {
			fn(b, setup(b, sc))
		})
	}
}

func BenchmarkGet(b *testing.B) {
	forEachSpreadBench(b, func(b *testing.B, h *harness) {
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
	})
}

func BenchmarkSet(b *testing.B) {
	forEachSpreadBench(b, func(b *testing.B, h *harness) {
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := h.eng.Set(ctx, nodeAddr, tagSetting, int32(i)); err != nil {
				b.Fatalf("Set: %v", err)
			}
		}
		b.StopTimer()
		reportLatency(b)
	})
}

// reportLatency adds a human-friendly ms/op metric alongside the default ns/op.
func reportLatency(b *testing.B) {
	if b.N > 0 {
		nsPerOp := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
		b.ReportMetric(nsPerOp/1e6, "ms/op")
	}
}
