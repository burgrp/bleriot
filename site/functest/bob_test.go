//go:build dongles

// Package functest, bob harness: hardware-in-the-loop measurements against the
// real bob node (an actual PY32F030 board) using a SINGLE hub dongle, instead of
// the two-dongle hub/node loopback in dongles_test.go. This is the setup used to
// investigate spontaneous-push (Notify) reliability: only one physical dongle is
// driven as the hub, and the node under test is the flashed bob firmware.
//
// Run with the Far dongle path and bob on channel 37 / S8:
//
//	BLERIOT_DONGLE_HUB=mcp2210:/dev/hidraw8 \
//	    go test -tags dongles -v -run TestBob ./functest/...
package functest

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/burgrp/tinygo-drivers/pan211x"

	"github.com/burgrp/bleriot/site/engine"
	"github.com/burgrp/bleriot/site/mcp2210"
	"github.com/burgrp/bleriot/site/node"
	"github.com/burgrp/bleriot/site/radio"
	"github.com/burgrp/bleriot/site/radio/mcpdongle"
)

// bob's real identity, mirroring example/hub/main.go. The RF address is derived
// from the UID (CRC32), exactly as the hub does.
var (
	bobUID = [12]byte{0x5A, 0x33, 0x50, 0x41, 0x12, 0x32, 0x35, 0x32, 0x29, 0x93, 0x95, 0x00}
	bobKey = [node.KeyLen]byte{
		0x04, 0xB8, 0xAF, 0x87, 0x5D, 0x55, 0xFC, 0x76,
		0xAC, 0x96, 0x7F, 0xA7, 0x94, 0x20, 0x08, 0x22,
	}
)

// bob register tags (example/bob/spec).
const (
	bobRegLedGreen uint16 = 1
	bobRegLedRed   uint16 = 2
	bobRegGpio     uint16 = 3
)

// bobChannel / bobSpread fix bob to the Far channel and its S8 spreading factor.
const (
	bobChannel uint8 = 37
)

var bobSpread = pan211x.SpreadFactorS8

// setupBob opens the single hub dongle named by BLERIOT_DONGLE_HUB, brings up the
// engine on the Far channel, and registers the real bob node. It returns the
// engine and bob's derived address; the dongle is closed via tb.Cleanup. refresh
// sets the WATCH refresh interval: a long value isolates spontaneous pushes from
// solicited re-reads when measuring push loss.
func setupBob(tb testing.TB, retries int, refresh time.Duration) (*engine.Engine, [node.AddrLen]byte) {
	tb.Helper()
	hubEnv := os.Getenv("BLERIOT_DONGLE_HUB")
	if hubEnv == "" {
		tb.Skip("set BLERIOT_DONGLE_HUB to run bob hardware tests")
	}
	hubSel := mcpSelector(tb, "BLERIOT_DONGLE_HUB", hubEnv)

	hubDev, err := mcp2210.Open(hubSel)
	if err != nil {
		tb.Fatalf("open hub dongle %q: %v", hubSel, err)
	}
	hubD, err := mcpdongle.Open(hubDev, bobChannel, bobSpread, hubAddr)
	if err != nil {
		tb.Fatalf("hub dongle %q: %v", hubSel, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	hubRadio := radio.New(ctx, hubD)

	eng := engine.New(engine.Options{
		HubAddr:         hubAddr,
		Timeout:         opTimeout,
		Retries:         retries,
		RefreshInterval: refresh,
		LivenessMisses:  livenessMisses,
	})
	go eng.Run(ctx)
	if err := eng.AddRadio(ctx, bobChannel, hubRadio); err != nil {
		cancel()
		tb.Fatalf("AddRadio: %v", err)
	}

	addr := node.AddressFromUID(bobUID)
	n := node.NewNode("bob", bobChannel, &node.Descriptor{}, node.Identity{Address: addr, Key: bobKey})
	if err := eng.AddNode(n); err != nil {
		cancel()
		tb.Fatalf("add node: %v", err)
	}

	tb.Cleanup(func() {
		cancel()
		<-hubRadio.Done()
	})
	tb.Logf("bob address %02X%02X%02X%02X", addr[0], addr[1], addr[2], addr[3])
	return eng, addr
}

// TestBobGetBaseline measures solicited-transaction reliability and latency
// against the real bob: many GETs of RegGpio. With the engine's retransmissions
// enabled this reflects the *effective* reliability the hub sees for solicited
// traffic (GET/SET/WATCH), which is the baseline the lossy spontaneous-push path
// is compared against.
func TestBobGetBaseline(t *testing.T) {
	eng, addr := setupBob(t, opRetries, refreshInterval)

	const n = 200
	var ok, fail int
	var total, max time.Duration
	for i := 0; i < n; i++ {
		ctx, c := context.WithTimeout(context.Background(), 3*time.Second)
		start := time.Now()
		_, err := eng.Get(ctx, addr, bobRegGpio)
		d := time.Since(start)
		c()
		if err != nil {
			fail++
			t.Logf("GET %3d: error after %v: %v", i, d, err)
			continue
		}
		ok++
		total += d
		if d > max {
			max = d
		}
	}
	var avg time.Duration
	if ok > 0 {
		avg = total / time.Duration(ok)
	}
	t.Logf("GET RegGpio over %d ops (retries=%d): ok=%d fail=%d  avg=%v max=%v",
		n, opRetries, ok, fail, avg, max)
	if fail > 0 {
		t.Errorf("%d/%d effective GET failures — link or node unreliable even with retries", fail, n)
	}
}

// TestBobPushLoss measures spontaneous-push (Notify) reliability against a bob
// firmware built with the TEMP push-loss bench (an incrementing counter pushed
// on RegGpio every 200 ms). The hub WATCHes RegGpio once, then a long refresh
// interval keeps the engine from re-reading the register, so every value the
// callback sees is a spontaneous push. Gaps in the received counter sequence are
// lost pushes: the loss rate the unACKed push path actually suffers over RF.
func TestBobPushLoss(t *testing.T) {
	const window = 60 * time.Second
	// Long refresh so solicited re-reads don't fill gaps and mask push loss; the
	// initial WATCH still uses retries so the subscription itself lands.
	eng, addr := setupBob(t, opRetries, 10*time.Minute)

	var mu sync.Mutex
	seen := map[int32]bool{}
	var first, last int32
	var haveFirst, seeded bool
	cb := func(u engine.Update) {
		if u.Null {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		// The first delivered update is the WATCH initial IS reply, which carries
		// bob's real RegGpio pin state (not the bench counter) and would skew the
		// counter range. Drop it; only spontaneous Notify pushes follow.
		if !seeded {
			seeded = true
			return
		}
		if !haveFirst {
			first, haveFirst = u.Value, true
		}
		if u.Value > last {
			last = u.Value
		}
		seen[u.Value] = true
	}

	ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
	err := eng.Watch(ctx, addr, bobRegGpio, cb)
	c()
	if err != nil {
		t.Fatalf("WATCH RegGpio: %v", err)
	}

	t.Logf("watching bob RegGpio for %v (bob pushes a counter every 200ms)…", window)
	time.Sleep(window)

	mu.Lock()
	defer mu.Unlock()
	recv := len(seen)
	if recv == 0 {
		t.Fatalf("received 0 pushes in %v — subscription or link broken", window)
	}
	// Count gaps only inside the contiguously observed range [first, last] so the
	// pre-subscription warm-up doesn't count as loss.
	span := int(last-first) + 1
	lost := span - recv
	var lossPct float64
	if span > 0 {
		lossPct = 100 * float64(lost) / float64(span)
	}
	// List the missing counter values (capped) to reveal whether losses are
	// isolated singletons (RF PER) or clustered bursts (a timing window).
	var missing []int32
	for v := first; v <= last && len(missing) < 40; v++ {
		if !seen[v] {
			missing = append(missing, v)
		}
	}
	t.Logf("push loss: range [%d,%d] span=%d received=%d lost=%d loss=%.1f%%",
		first, last, span, recv, lost, lossPct)
	t.Logf("first 40 missing counters: %v", missing)
}

// TestBobPushLossUnderLoad measures push loss while the hub is actively
// transmitting (a tight GET loop) at the same time bob pushes its counter. Each
// hub transmit blanks the half-duplex dongle for the ~20 ms TX guard window
// (send: STB3→TX→poll→enterRX), during which an arriving spontaneous push is not
// received. This is the real deployment condition the idle-hub TestBobPushLoss
// does not exercise, and it reveals collision-driven loss the unACKed push path
// cannot recover from.
func TestBobPushLossUnderLoad(t *testing.T) {
	const window = 60 * time.Second
	eng, addr := setupBob(t, opRetries, 10*time.Minute)

	var mu sync.Mutex
	seen := map[int32]bool{}
	var first, last int32
	var haveFirst, seeded bool
	cb := func(u engine.Update) {
		if u.Null {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if !seeded {
			seeded = true
			return
		}
		if !haveFirst {
			first, haveFirst = u.Value, true
		}
		if u.Value > last {
			last = u.Value
		}
		seen[u.Value] = true
	}

	ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
	if err := eng.Watch(ctx, addr, bobRegGpio, cb); err != nil {
		c()
		t.Fatalf("WATCH RegGpio: %v", err)
	}
	c()

	// Background GET loop on a different register (RegLedRed) so the hub keeps
	// transmitting throughout the window, colliding with bob's pushes.
	loadCtx, stop := context.WithCancel(context.Background())
	var gets, getErr int
	var loadWG sync.WaitGroup
	loadWG.Add(1)
	go func() {
		defer loadWG.Done()
		for loadCtx.Err() == nil {
			gctx, gc := context.WithTimeout(loadCtx, 2*time.Second)
			_, err := eng.Get(gctx, addr, bobRegLedRed)
			gc()
			mu.Lock()
			gets++
			if err != nil && loadCtx.Err() == nil {
				getErr++
			}
			mu.Unlock()
		}
	}()

	t.Logf("watching bob RegGpio under concurrent GET load for %v…", window)
	time.Sleep(window)
	stop()
	loadWG.Wait()

	mu.Lock()
	defer mu.Unlock()
	recv := len(seen)
	if recv == 0 {
		t.Fatalf("received 0 pushes in %v — subscription or link broken", window)
	}
	span := int(last-first) + 1
	lost := span - recv
	var lossPct float64
	if span > 0 {
		lossPct = 100 * float64(lost) / float64(span)
	}
	var missing []int32
	for v := first; v <= last && len(missing) < 40; v++ {
		if !seen[v] {
			missing = append(missing, v)
		}
	}
	t.Logf("hub GETs during window: %d (errors %d)", gets, getErr)
	t.Logf("push loss under load: range [%d,%d] span=%d received=%d lost=%d loss=%.1f%%",
		first, last, span, recv, lost, lossPct)
	t.Logf("first 40 missing counters: %v", missing)
}
