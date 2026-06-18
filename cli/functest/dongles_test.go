//go:build dongles

// Package functest contains hardware-in-the-loop functional tests for the whole
// BleRiot protocol stack. They run the real hub engine on one USB dongle and the
// real node runtime (protocol/node) on a second USB dongle, exchanging packets
// over the air with no microcontroller and no mocks — exercising the XTEA codec,
// packet framing, GET/SET/WATCH transactions, retries, push subscriptions, and
// liveness detection end to end.
//
// These tests are gated behind the "dongles" build tag and require two MCP2210
// dongles. Run them with:
//
//	BLERIOT_DONGLE_HUB=mcp2210:/dev/hidraw3 BLERIOT_DONGLE_NODE=mcp2210:/dev/hidraw4 \
//	    go test -tags dongles -v ./functest/...
//
// Each dongle env var is "scheme:selector"; the scheme is required (only
// "mcp2210" is supported here, mirroring the hub --dongle flag, which has no
// default) and the selector is a /dev/hidraw* path or a USB serial string (see
// mcp2210.Open). BLERIOT_CHANNEL (default 37) sets the shared BLE channel. When
// the dongle env vars are unset the tests skip, so a normal `go test ./...` and
// CI are unaffected.
package functest

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	pnode "protocol/node"

	"cli/pkg/engine"
	"cli/pkg/mcp2210"
	"cli/pkg/node"
	"cli/pkg/radio"
	"cli/pkg/radio/mcpdongle"
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
	tagPush    uint16 = 3
	tagUnknown uint16 = 99
)

// Engine tuning for the relatively slow USB-HID round trips: each protocol
// transaction is many SPI ops on both dongles, so timeouts are generous and the
// refresh interval is short enough to detect a silent node within a few seconds.
const (
	opTimeout       = 500 * time.Millisecond
	opRetries       = 3
	refreshInterval = 200 * time.Millisecond
	livenessMisses  = 2
	waitTimeout     = 8 * time.Second
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
// selector to pass to mcp2210.Open. Keeping the scheme explicit mirrors the hub
// --dongle flag, which has no default scheme.
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

// sensorEvent is an out-of-band register change applied inside the node loop so
// that node.Notify is only ever called from the node's own goroutine.
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
	nrt  *pnode.Node
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
	// A SET that lands is also pushed to any watcher, like a real device whose
	// register settled to the written value.
	d.nrt.Notify(tag, value, null)
}

// apply mutates the stored value without notifying (used both for the initial
// seed and for simulated sensor changes processed in the node loop).
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
	cancel  context.CancelFunc
	eng     *engine.Engine
	dev     *memDevice
	sensor  chan sensorEvent
	paused  chan bool
	noderad *radio.NodeRadio
	wg      sync.WaitGroup
}

func setup(tb testing.TB) *harness {
	tb.Helper()
	hubEnv := os.Getenv("BLERIOT_DONGLE_HUB")
	nodeEnv := os.Getenv("BLERIOT_DONGLE_NODE")
	if hubEnv == "" || nodeEnv == "" {
		tb.Skip("set BLERIOT_DONGLE_HUB and BLERIOT_DONGLE_NODE to run dongle functional tests")
	}
	hubSel := mcpSelector(tb, "BLERIOT_DONGLE_HUB", hubEnv)
	nodeSel := mcpSelector(tb, "BLERIOT_DONGLE_NODE", nodeEnv)
	channel := uint8(37)
	if s := os.Getenv("BLERIOT_CHANNEL"); s != "" {
		v, err := strconv.ParseUint(s, 10, 8)
		if err != nil {
			tb.Fatalf("BLERIOT_CHANNEL: %v", err)
		}
		channel = uint8(v)
	}

	hubDev, err := mcp2210.Open(hubSel)
	if err != nil {
		tb.Fatalf("open hub dongle %q: %v", hubSel, err)
	}
	hubD, err := mcpdongle.Open(hubDev, channel, hubAddr)
	if err != nil {
		tb.Fatalf("hub dongle %q: %v", hubSel, err) // Open closed hubDev
	}
	nodeDev, err := mcp2210.Open(nodeSel)
	if err != nil {
		hubD.Close()
		tb.Fatalf("open node dongle %q: %v", nodeSel, err)
	}
	nodeD, err := mcpdongle.Open(nodeDev, channel, nodeAddr)
	if err != nil {
		hubD.Close()
		tb.Fatalf("node dongle %q: %v", nodeSel, err) // Open closed nodeDev
	}

	ctx, cancel := context.WithCancel(context.Background())

	// The hub radio's receive loop owns hubD and closes it when ctx is cancelled;
	// the node radio owns nodeD and closes it on nodeRadio.Close().
	hubRadio := radio.New(ctx, hubD)
	nodeRadio := radio.NewNode(nodeD)

	eng := engine.New(engine.Options{
		HubAddr:         hubAddr,
		Timeout:         opTimeout,
		Retries:         opRetries,
		RefreshInterval: refreshInterval,
		LivenessMisses:  livenessMisses,
	})
	go eng.Run(ctx)
	eng.AddRadio(ctx, channel, hubRadio)

	n := node.NewNode("functest", channel, &node.Descriptor{}, node.Identity{Address: nodeAddr, Key: nodeKey})
	if err := eng.AddNode(n); err != nil {
		cancel()
		nodeRadio.Close()
		tb.Fatalf("add node: %v", err)
	}

	dev := newMemDevice()
	nrt, err := pnode.New(nodeRadio, nodeAddr, nodeKey, dev)
	if err != nil {
		cancel()
		nodeRadio.Close()
		tb.Fatalf("node runtime: %v", err)
	}
	dev.nrt = nrt

	h := &harness{
		cancel:  cancel,
		eng:     eng,
		dev:     dev,
		sensor:  make(chan sensorEvent, 8),
		paused:  make(chan bool, 1),
		noderad: nodeRadio,
	}

	// Node loop: drains simulated sensor events and polls the runtime. Pausing
	// makes the node stop answering, which is how the liveness test takes it
	// "offline" without unplugging anything.
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		paused := false
		for {
			if ctx.Err() != nil {
				return
			}
			select {
			case p := <-h.paused:
				paused = p
			case ev := <-h.sensor:
				dev.apply(ev)
				nrt.Notify(ev.tag, ev.value, ev.null)
			default:
			}
			if paused {
				time.Sleep(5 * time.Millisecond)
				continue
			}
			nrt.Poll()
		}
	}()

	tb.Cleanup(func() {
		cancel()
		h.wg.Wait()
		// Wait for the hub receive loop to exit and close its dongle, then close
		// the node dongle, so neither physical device is still in use when the
		// next test re-opens it (overlapping sessions desync the HID stream).
		<-hubRadio.Done()
		nodeRadio.Close()
	})
	return h
}

// seed sets a register value before exercising the stack.
func (h *harness) seed(tag uint16, value int32) {
	h.dev.apply(sensorEvent{tag: tag, value: value})
}

// pushSensor simulates an autonomous register change on the node.
func (h *harness) pushSensor(tag uint16, value int32) {
	h.sensor <- sensorEvent{tag: tag, value: value}
}

// setPaused toggles whether the node answers the radio.
func (h *harness) setPaused(p bool) {
	h.paused <- p
}

func opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), waitTimeout)
}

func TestGet(t *testing.T) {
	h := setup(t)
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
}

func TestGetUnknownIsNull(t *testing.T) {
	h := setup(t)

	ctx, cancel := opCtx()
	defer cancel()
	u, err := h.eng.Get(ctx, nodeAddr, tagUnknown)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !u.Null {
		t.Fatalf("Get unknown tag = %+v, want null", u)
	}
}

func TestSetThenGet(t *testing.T) {
	h := setup(t)

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
}

func TestSetNullClears(t *testing.T) {
	h := setup(t)
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
}

func TestWatchReceivesInitialAndPush(t *testing.T) {
	h := setup(t)
	h.seed(tagPush, 100)

	updates := make(chan engine.Update, 8)
	ctx, cancel := opCtx()
	defer cancel()
	if err := h.eng.Watch(ctx, nodeAddr, tagPush, func(u engine.Update) {
		updates <- u
	}); err != nil {
		t.Fatalf("Watch: %v", err)
	}

	// Initial IS reply carries the current value.
	if u := waitUpdate(t, updates, func(u engine.Update) bool { return !u.Null && u.Value == 100 }); u.Value != 100 {
		t.Fatalf("initial update = %+v, want 100", u)
	}

	// An autonomous change is pushed to the watcher.
	h.pushSensor(tagPush, 200)
	if u := waitUpdate(t, updates, func(u engine.Update) bool { return !u.Null && u.Value == 200 }); u.Value != 200 {
		t.Fatalf("pushed update = %+v, want 200", u)
	}
}

func TestLivenessNullOnSilentNode(t *testing.T) {
	h := setup(t)
	h.seed(tagPush, 7)

	updates := make(chan engine.Update, 8)
	ctx, cancel := opCtx()
	defer cancel()
	if err := h.eng.Watch(ctx, nodeAddr, tagPush, func(u engine.Update) {
		updates <- u
	}); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	// Drain the initial live value.
	waitUpdate(t, updates, func(u engine.Update) bool { return !u.Null })

	// Take the node offline: refreshes now time out, and after livenessMisses
	// the engine must report the register as NULL.
	h.setPaused(true)
	if u := waitUpdate(t, updates, func(u engine.Update) bool { return u.Null }); !u.Null {
		t.Fatalf("offline update = %+v, want null", u)
	}

	// Bringing the node back must restore a live value to the watcher.
	h.setPaused(false)
	if u := waitUpdate(t, updates, func(u engine.Update) bool { return !u.Null && u.Value == 7 }); u.Value != 7 {
		t.Fatalf("recovered update = %+v, want 7", u)
	}
}

// waitUpdate waits for an update matching pred or fails after waitTimeout.
func waitUpdate(t *testing.T, ch <-chan engine.Update, pred func(engine.Update) bool) engine.Update {
	t.Helper()
	deadline := time.After(waitTimeout)
	for {
		select {
		case u := <-ch:
			if pred(u) {
				return u
			}
		case <-deadline:
			t.Fatalf("timed out waiting for matching update")
			return engine.Update{}
		}
	}
}
