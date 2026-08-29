//go:build dongles

// Package functest, bob harness: hardware-in-the-loop measurements against the
// real bob node (an actual PY32F030 board) using a single hub dongle, instead of
// the two-dongle hub/node loopback in dongles_test.go.
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

	"github.com/burgrp/bleriot/lib/site/engine"
	"github.com/burgrp/bleriot/lib/site/mcp2210"
	"github.com/burgrp/bleriot/lib/site/node"
	"github.com/burgrp/bleriot/lib/site/radio"
	"github.com/burgrp/bleriot/lib/site/radio/mcpdongle"
)

// bob's real identity, mirroring example/bob/test-hub.go.
var (
	bobAddress = [node.AddrLen]byte{0xCC, 0x81, 0xAF, 0x84}
	bobKey     = [node.KeyLen]byte{
		0x04, 0xB8, 0xAF, 0x87, 0x5D, 0x55, 0xFC, 0x76,
		0xAC, 0x96, 0x7F, 0xA7, 0x94, 0x20, 0x08, 0x22,
	}
	benchAddress = [node.AddrLen]byte{0xAE, 0x4D, 0xB3, 0x50}
	benchKey     = [node.KeyLen]byte{
		0x72, 0x28, 0x7D, 0xBA, 0x69, 0x31, 0x5A, 0x3E,
		0xA0, 0xC3, 0x26, 0x77, 0x43, 0xB0, 0x3E, 0xAC,
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

type realBob struct {
	name    string
	address [node.AddrLen]byte
	key     [node.KeyLen]byte
	channel uint8
	spread  pan211x.SpreadFactor
}

var realBobs = []realBob{
	{name: "bob", address: bobAddress, key: bobKey, channel: 37, spread: pan211x.SpreadFactorS8},
	{name: "bench", address: benchAddress, key: benchKey, channel: 38, spread: pan211x.SpreadFactorS2},
}

// setupBob opens the single hub dongle named by BLERIOT_DONGLE_HUB, brings up the
// engine on the Far channel, and registers the real bob node. It returns the
// engine and bob's address; the dongle is closed via tb.Cleanup.
func setupBob(tb testing.TB, retries int) (*engine.Engine, [node.AddrLen]byte) {
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
	tb.Cleanup(func() {
		cancel()
		<-hubRadio.Done()
	})

	eng := engine.New(engine.Options{
		HubAddr: hubAddr,
		Timeout: opTimeout,
		Retries: retries,
	})
	if err := eng.AddRadio(ctx, bobChannel, hubRadio); err != nil {
		tb.Fatalf("AddRadio: %v", err)
	}

	addr := bobAddress
	n := node.NewNode("bob", bobChannel, &node.Descriptor{}, node.Identity{Address: addr, Key: bobKey})
	if err := eng.AddNode(n); err != nil {
		tb.Fatalf("add node: %v", err)
	}
	tb.Logf("bob address %02X%02X%02X%02X", addr[0], addr[1], addr[2], addr[3])
	return eng, addr
}

// TestBobGetBaseline measures transaction reliability and latency against the
// real bob with repeated GETs of RegGpio.
func TestBobGetBaseline(t *testing.T) {
	eng, addr := setupBob(t, opRetries)

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
		t.Errorf("%d/%d effective GET failures; link or node unreliable even with retries", fail, n)
	}
}

func TestBobGroupsConcurrent(t *testing.T) {
	if os.Getenv("BLERIOT_DONGLE_FAR") == "" || os.Getenv("BLERIOT_DONGLE_NEAR") == "" {
		t.Skip("set BLERIOT_DONGLE_FAR and BLERIOT_DONGLE_NEAR to run two-group BOB tests")
	}
	selectors := []string{
		mcpSelector(t, "BLERIOT_DONGLE_FAR", os.Getenv("BLERIOT_DONGLE_FAR")),
		mcpSelector(t, "BLERIOT_DONGLE_NEAR", os.Getenv("BLERIOT_DONGLE_NEAR")),
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	eng := engine.New(engine.Options{HubAddr: hubAddr, Timeout: opTimeout, Retries: opRetries})
	for index, board := range realBobs {
		device, err := mcp2210.Open(selectors[index])
		if err != nil {
			cancel()
			t.Fatalf("open %s dongle %q: %v", board.name, selectors[index], err)
		}
		dongle, err := mcpdongle.Open(device, board.channel, board.spread, hubAddr)
		if err != nil {
			t.Fatalf("configure %s dongle: %v", board.name, err)
		}
		channelRadio := radio.New(ctx, dongle)
		t.Cleanup(func() {
			cancel()
			<-channelRadio.Done()
		})
		if err := eng.AddRadio(ctx, board.channel, channelRadio); err != nil {
			t.Fatalf("add %s radio: %v", board.name, err)
		}
		descriptor := &node.Descriptor{Registers: []node.Register{
			{ID: bobRegLedGreen, Name: "green"},
			{ID: bobRegLedRed, Name: "red"},
			{ID: bobRegGpio, Name: "gpio", ReadOnly: true},
		}}
		if err := eng.AddNode(node.NewNode(board.name, board.channel, descriptor, node.Identity{Address: board.address, Key: board.key})); err != nil {
			t.Fatalf("add %s node: %v", board.name, err)
		}
	}

	const operations = 50
	for iteration := 0; iteration < operations; iteration++ {
		var wait sync.WaitGroup
		errors := make(chan error, len(realBobs))
		for _, board := range realBobs {
			board := board
			wait.Add(1)
			go func() {
				defer wait.Done()
				requestCtx, requestCancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer requestCancel()
				if _, err := eng.Get(requestCtx, board.address, bobRegGpio); err != nil {
					errors <- err
				}
			}()
		}
		wait.Wait()
		close(errors)
		for err := range errors {
			t.Fatalf("concurrent GET iteration %d: %v", iteration, err)
		}
	}

	wanted := []int32{137, 211}
	defaults := []int32{500, 100}
	for index, board := range realBobs {
		requestCtx, requestCancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := eng.Set(requestCtx, board.address, bobRegLedGreen, wanted[index]); err != nil {
			requestCancel()
			t.Fatalf("SET %s green: %v", board.name, err)
		}
		update, err := eng.Get(requestCtx, board.address, bobRegLedGreen)
		requestCancel()
		if err != nil {
			t.Fatalf("GET %s green after SET: %v", board.name, err)
		}
		if update.Null || update.Value != wanted[index] {
			t.Fatalf("%s green = %+v, want %d", board.name, update, wanted[index])
		}
		requestCtx, requestCancel = context.WithTimeout(context.Background(), 3*time.Second)
		if err := eng.SetNull(requestCtx, board.address, bobRegLedGreen); err != nil {
			requestCancel()
			t.Fatalf("SET NULL %s green: %v", board.name, err)
		}
		update, err = eng.Get(requestCtx, board.address, bobRegLedGreen)
		requestCancel()
		if err != nil {
			t.Fatalf("GET %s green after SET NULL: %v", board.name, err)
		}
		if update.Null || update.Value != 0 {
			t.Fatalf("%s green after SET NULL = %+v, want value 0", board.name, update)
		}
		requestCtx, requestCancel = context.WithTimeout(context.Background(), 3*time.Second)
		if err := eng.Set(requestCtx, board.address, bobRegLedGreen, defaults[index]); err != nil {
			requestCancel()
			t.Fatalf("restore %s green: %v", board.name, err)
		}
		requestCancel()
	}
}
