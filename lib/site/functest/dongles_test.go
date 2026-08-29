//go:build dongles

// Package functest contains hardware-in-the-loop functional tests for the whole
// BleRiot protocol stack. They run the real hub engine on one USB dongle and the
// real node runtime (protocol/node) on a second USB dongle, exchanging packets
// over the air with no microcontroller and no mocks — exercising the XTEA codec,
// packet framing, GET/SET transactions, retries, and the reply turnaround guard
// end to end.
//
// Every test runs twice, once per BLE Coded PHY spreading factor: S8 (long
// range, ~125 kbps) on channel 37 and S2 (shorter range, ~500 kbps) on channel
// 38, mirroring the example inventory's Far/Near channels. The spreading factor
// is never selected by an environment variable — both are always covered, as
// subtests named "S8" and "S2".
//
// These tests are gated behind the "dongles" build tag and require two MCP2210
// dongles. Run them with:
//
//	BLERIOT_DONGLE_HUB=mcp2210:/dev/hidraw3 BLERIOT_DONGLE_NODE=mcp2210:/dev/hidraw4 \
//	    go test -tags dongles -v ./functest/...
//
// Each dongle env var is "scheme:selector": the scheme is required (only
// "mcp2210" is supported here) and the selector is a /dev/hidraw* path or a USB
// serial string (see mcp2210.Open). The two env vars only say *which* physical
// dongles to use; the
// channel and spreading factor are fixed by the test matrix (see spreadConfigs).
// When the dongle env vars are unset the tests skip, so a normal `go test ./...`
// and CI are unaffected.
package functest

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	pnode "github.com/burgrp/bleriot/lib/node"

	"github.com/burgrp/tinygo-drivers/pan211x"

	"github.com/burgrp/bleriot/lib/site/engine"
	"github.com/burgrp/bleriot/lib/site/mcp2210"
	"github.com/burgrp/bleriot/lib/site/node"
	"github.com/burgrp/bleriot/lib/site/radio"
	"github.com/burgrp/bleriot/lib/site/radio/mcpdongle"
)

// Fixed test identities. The hub and node listen on distinct RF addresses on the
// same channel; the shared XTEA key authenticates the link.
var (
	hubAddr  = mustAddr("FFFFFF01")
	nodeAddr = mustAddr("DEADBEEF")
	nodeKey  = [node.KeyLen]byte{
		0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF,
	}
)

// Test register tags exposed by the in-memory node device.
const (
	tagTemp    uint16 = 1
	tagSetting uint16 = 2
	tagUnknown uint16 = 99
)

// spreadConfig pairs a spreading factor with the channel it is tested on. Every
// test runs against each entry so both BLE Coded PHY factors are exercised over
// real RF, named as subtests "S8" and "S2".
type spreadConfig struct {
	name    string
	channel uint8
	spread  pan211x.SpreadFactor
}

// spreadConfigs is the full test matrix: S8 (long range) on channel 37 and S2
// (faster, shorter range) on channel 38, matching the example inventory's Far
// and Near channels. There is deliberately no environment-variable override —
// both factors are always covered.
var spreadConfigs = []spreadConfig{
	{name: "S8", channel: 37, spread: pan211x.SpreadFactorS8},
	{name: "S2", channel: 38, spread: pan211x.SpreadFactorS2},
}

// Engine tuning for the relatively slow USB-HID round trips: each protocol
// transaction is many SPI register accesses on both dongles, and on the MCP2210
// "dumb" dongle every register access is a full USB-HID round trip (~1 ms at USB
// full speed). The per-attempt timeout is therefore generous, and it comfortably
// clears the dongle's reply turnaround guard (mcpdongle.replyGuard, ~20 ms) plus
// the engine's minReplyHeadroom, so a deferred reply always lands inside the
// response window.
const (
	opTimeout   = 500 * time.Millisecond
	opRetries   = 3
	waitTimeout = 8 * time.Second
)

func mustAddr(s string) [node.AddrLen]byte {
	a, err := node.ParseAddress(s)
	if err != nil {
		panic(err)
	}
	return a
}

// mcpSelector requires a dongle env value to carry the "mcp2210:" scheme (the
// only dongle type these hardware tests support) and returns the bare device
// selector to pass to mcp2210.Open. The scheme is kept explicit so the env var
// names which dongle type to use, with no implicit default.
func mcpSelector(tb testing.TB, env, val string) string {
	tb.Helper()
	i := strings.Index(val, ":")
	if i < 0 {
		tb.Fatalf("%s=%q: expected scheme:selector, e.g. mcp2210:0001746423", env, val)
	}
	scheme, sel := val[:i], val[i+1:]
	if scheme != "mcp2210" {
		tb.Fatalf("%s=%q: unsupported dongle scheme %q (only mcp2210)", env, val, scheme)
	}
	if sel == "" {
		tb.Fatalf("%s=%q: empty selector", env, val)
	}
	return sel
}

type sensorEvent struct {
	tag   uint16
	value int32
	null  bool
}

// memDevice is a trivial in-memory node.Device: a tag→value map with per-tag
// null state. Unknown tags read as null. Every method runs on the node loop
// goroutine, so it needs no locking, but a mutex guards the seed path used
// before the loop starts.
type memDevice struct {
	mu   sync.Mutex
	vals map[uint16]int32
	null map[uint16]bool
}

func newMemDevice() *memDevice {
	return &memDevice{vals: map[uint16]int32{}, null: map[uint16]bool{}}
}

func (d *memDevice) Read(tag uint16) (int32, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.null[tag] {
		return 0, true
	}
	v, ok := d.vals[tag]
	if !ok {
		return 0, true // unknown tag → null
	}
	return v, false
}

func (d *memDevice) Write(tag uint16, value int32, null bool) {
	d.apply(sensorEvent{tag: tag, value: value, null: null})
}

// apply mutates the stored value.
func (d *memDevice) apply(ev sensorEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if ev.null {
		d.null[ev.tag] = true
		delete(d.vals, ev.tag)
		return
	}
	d.null[ev.tag] = false
	d.vals[ev.tag] = ev.value
}

// harness owns both dongles, the hub engine, and the node runtime for the test.
type harness struct {
	eng *engine.Engine
	dev *memDevice
	wg  sync.WaitGroup
}

func setup(tb testing.TB, sc spreadConfig) *harness {
	tb.Helper()
	hubEnv := os.Getenv("BLERIOT_DONGLE_HUB")
	nodeEnv := os.Getenv("BLERIOT_DONGLE_NODE")
	if hubEnv == "" || nodeEnv == "" {
		tb.Skip("set BLERIOT_DONGLE_HUB and BLERIOT_DONGLE_NODE to run dongle functional tests")
	}
	hubSel := mcpSelector(tb, "BLERIOT_DONGLE_HUB", hubEnv)
	nodeSel := mcpSelector(tb, "BLERIOT_DONGLE_NODE", nodeEnv)
	channel := sc.channel
	spread := sc.spread

	hubDev, err := mcp2210.Open(hubSel)
	if err != nil {
		tb.Fatalf("open hub dongle %q: %v", hubSel, err)
	}
	hubD, err := mcpdongle.Open(hubDev, channel, spread, hubAddr)
	if err != nil {
		tb.Fatalf("hub dongle %q: %v", hubSel, err) // Open closed hubDev
	}
	nodeDev, err := mcp2210.Open(nodeSel)
	if err != nil {
		hubD.Close()
		tb.Fatalf("open node dongle %q: %v", nodeSel, err)
	}
	nodeD, err := mcpdongle.Open(nodeDev, channel, spread, nodeAddr)
	if err != nil {
		hubD.Close()
		tb.Fatalf("node dongle %q: %v", nodeSel, err) // Open closed nodeDev
	}

	ctx, cancel := context.WithCancel(context.Background())

	// The hub radio's receive loop owns hubD and closes it when ctx is cancelled;
	// the node radio owns nodeD and closes it on nodeRadio.Close().
	hubRadio := radio.New(ctx, hubD)
	tb.Cleanup(func() {
		cancel()
		<-hubRadio.Done()
	})
	nodeRadio := radio.NewNode(nodeD)
	tb.Cleanup(func() { nodeRadio.Close() })

	eng := engine.New(engine.Options{
		HubAddr: hubAddr,
		Timeout: opTimeout,
		Retries: opRetries,
	})
	if err := eng.AddRadio(ctx, channel, hubRadio); err != nil {
		tb.Fatalf("AddRadio: %v", err)
	}

	n := node.NewNode("functest", channel, &node.Descriptor{}, node.Identity{Address: nodeAddr, Key: nodeKey})
	if err := eng.AddNode(n); err != nil {
		tb.Fatalf("add node: %v", err)
	}

	dev := newMemDevice()
	nrt, err := pnode.New(nodeRadio, nodeAddr, nodeKey, dev)
	if err != nil {
		tb.Fatalf("node runtime: %v", err)
	}
	h := &harness{
		eng: eng,
		dev: dev,
	}

	// Node loop polls the runtime for hub-initiated transactions.
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		for ctx.Err() == nil {
			nrt.Poll()
		}
	}()

	tb.Cleanup(func() {
		cancel()
		h.wg.Wait()
	})
	return h
}

// seed sets a register value before exercising the stack.
func (h *harness) seed(tag uint16, value int32) {
	h.dev.apply(sensorEvent{tag: tag, value: value})
}

func opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), waitTimeout)
}

// forEachSpread runs fn once per spreading factor in spreadConfigs, each as a
// named subtest ("S8", "S2") with a freshly set-up harness on that factor's
// channel. This is how every functional test covers both BLE Coded PHY factors
// without any environment-variable selection.
func forEachSpread(t *testing.T, fn func(t *testing.T, h *harness)) {
	t.Helper()
	for _, sc := range spreadConfigs {
		t.Run(sc.name, func(t *testing.T) {
			fn(t, setup(t, sc))
		})
	}
}

func TestGet(t *testing.T) {
	forEachSpread(t, func(t *testing.T, h *harness) {
		h.seed(tagTemp, 2137)

		ctx, cancel := opCtx()
		defer cancel()
		u, err := h.eng.Get(ctx, nodeAddr, tagTemp)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if u.Null || u.Value != 2137 {
			t.Fatalf("Get = %+v, want value 2137", u)
		}
	})
}

func TestGetUnknownIsNull(t *testing.T) {
	forEachSpread(t, func(t *testing.T, h *harness) {
		ctx, cancel := opCtx()
		defer cancel()
		u, err := h.eng.Get(ctx, nodeAddr, tagUnknown)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !u.Null {
			t.Fatalf("Get unknown tag = %+v, want null", u)
		}
	})
}

func TestSetThenGet(t *testing.T) {
	forEachSpread(t, func(t *testing.T, h *harness) {
		ctx, cancel := opCtx()
		defer cancel()
		if err := h.eng.Set(ctx, nodeAddr, tagSetting, 555); err != nil {
			t.Fatalf("Set: %v", err)
		}
		u, err := h.eng.Get(ctx, nodeAddr, tagSetting)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if u.Null || u.Value != 555 {
			t.Fatalf("Get after Set = %+v, want value 555", u)
		}
	})
}

func TestSetNullClears(t *testing.T) {
	forEachSpread(t, func(t *testing.T, h *harness) {
		h.seed(tagSetting, 12)

		ctx, cancel := opCtx()
		defer cancel()
		if err := h.eng.SetNull(ctx, nodeAddr, tagSetting); err != nil {
			t.Fatalf("SetNull: %v", err)
		}
		u, err := h.eng.Get(ctx, nodeAddr, tagSetting)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !u.Null {
			t.Fatalf("Get after SetNull = %+v, want null", u)
		}
	})
}
