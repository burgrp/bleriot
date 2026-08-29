package main

import (
	"sync/atomic"
	"testing"
)

func TestWritePeriod(t *testing.T) {
	var period atomic.Int32
	writePeriod(&period, 250, false)
	if got := period.Load(); got != 250 {
		t.Fatalf("ordinary period = %d, want 250", got)
	}
	writePeriod(&period, 999, true)
	if got := period.Load(); got != 0 {
		t.Fatalf("NULL period = %d, want off (0)", got)
	}
}
