package cli

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/burgrp/bleriot/lib/shared/config"
	"github.com/burgrp/bleriot/lib/shared/inventory"
	"github.com/burgrp/bleriot/lib/site/engine"
	"github.com/burgrp/bleriot/lib/site/node"
	"github.com/burgrp/bleriot/lib/site/radio"
)

// fakeProbe is an in-memory Probe for tests.
type fakeProbe struct {
	uid      [config.UIDLen]byte
	readErr  error
	written  []byte
	writeErr error
}

func (f *fakeProbe) ReadUID(context.Context) ([config.UIDLen]byte, error) {
	return f.uid, f.readErr
}

func (f *fakeProbe) WritePage(_ context.Context, image []byte) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.written = append([]byte(nil), image...)
	return nil
}

type bobConfig struct {
	DefaultRedPeriod   uint32
	DefaultGreenPeriod uint32
}

func sampleType() inventory.DeviceType {
	return inventory.DeviceType{
		Name: "bob",
		Chip: inventory.PY32F030x8,
		Registers: []inventory.Register{
			{Tag: 1, Name: "green", Type: inventory.TypeInt, Multiplier: 1, Divider: 1},
			{Tag: 2, Name: "red", Type: inventory.TypeInt, Multiplier: 1, Divider: 1},
		},
	}
}

func sampleInstance() inventory.Instance {
	return inventory.Instance{
		Name:    "kitchen",
		UID:     [config.UIDLen]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
		Key:     [config.KeyLen]byte{0xAA, 0xBB, 0xCC, 0xDD},
		Channel: inventory.Channel{Name: "near", Number: 37, SpreadFactor: config.SpreadFactorS2},
		Type:    sampleType(),
		Config:  bobConfig{DefaultRedPeriod: 500, DefaultGreenPeriod: 100},
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestBuildNode(t *testing.T) {
	inst := sampleInstance()

	n, err := buildNode(inst)
	if err != nil {
		t.Fatalf("buildNode: %v", err)
	}
	if n.Name != "kitchen" || n.Channel != 37 {
		t.Fatalf("node name/channel = %q/%d", n.Name, n.Channel)
	}
	if n.Address != node.AddressFromUID(inst.UID) {
		t.Fatalf("node address not derived from UID")
	}
	if n.Key != inst.Key {
		t.Fatalf("node key mismatch")
	}
	if r, ok := n.ByID(1); !ok || r.Name != "green" {
		t.Fatalf("register tag 1 not mapped: %v %v", r, ok)
	}
	if _, ok := n.ByID(2); !ok {
		t.Fatalf("register tag 2 not mapped")
	}
}

func TestRunProvisionWritesPage(t *testing.T) {
	inst := sampleInstance()
	inv := inventory.Inventory{inst}
	fp := &fakeProbe{uid: inst.UID}

	if err := runProvision(context.Background(), inv, inventory.PY32F030x8, fp, discardLogger()); err != nil {
		t.Fatalf("runProvision: %v", err)
	}
	if fp.written == nil {
		t.Fatal("no page written")
	}

	var got bobConfig
	h, err := config.Unmarshal(fp.written, &got)
	if err != nil {
		t.Fatalf("Unmarshal written page: %v", err)
	}
	if h.Address != node.AddressFromUID(inst.UID) {
		t.Fatalf("page address not derived from UID")
	}
	if h.Key != inst.Key {
		t.Fatalf("page key mismatch")
	}
	if h.Channel != inst.Channel.Number {
		t.Fatalf("page channel = %d, want %d", h.Channel, inst.Channel.Number)
	}
	if h.SpreadFactor != inst.Channel.SpreadFactor {
		t.Fatalf("page spread factor = %d, want %d", h.SpreadFactor, inst.Channel.SpreadFactor)
	}
	if got != inst.Config {
		t.Fatalf("page config = %+v, want %+v", got, inst.Config)
	}
}

func TestRunProvisionUnknownUID(t *testing.T) {
	inv := inventory.Inventory{sampleInstance()}
	fp := &fakeProbe{uid: [config.UIDLen]byte{0xFF}} // not in inventory

	err := runProvision(context.Background(), inv, inventory.PY32F030x8, fp, discardLogger())
	if err == nil {
		t.Fatal("expected error for unknown UID")
	}
	if fp.written != nil {
		t.Fatal("must not write a page for an unknown device")
	}
}

func TestRunNewPrintsStub(t *testing.T) {
	inv := inventory.Inventory{sampleInstance()}
	uid := [config.UIDLen]byte{0x10, 0x20, 0x30}
	fp := &fakeProbe{uid: uid}

	var buf bytes.Buffer
	if err := runNew(context.Background(), inv, fp, &buf, discardLogger()); err != nil {
		t.Fatalf("runNew: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Name:    \"TODO\"") {
		t.Fatalf("stub missing Instance literal:\n%s", out)
	}
	if !strings.Contains(out, "0x10, 0x20, 0x30") {
		t.Fatalf("stub missing UID bytes:\n%s", out)
	}
	// The stub must carry a freshly generated, non-zero key rather than a
	// placeholder.
	if strings.Contains(out, "TODO: 16-byte") {
		t.Fatalf("stub still has a key placeholder:\n%s", out)
	}
	if !strings.Contains(out, "Key:     [16]byte{") {
		t.Fatalf("stub missing key literal:\n%s", out)
	}
	if strings.Contains(out, "Key:     [16]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}") {
		t.Fatalf("generated key is all zero:\n%s", out)
	}
}

// stubDongle is a radio.Dongle that does nothing, for exercising the dongle
// assignment path without hardware.
type stubDongle struct{}

func (stubDongle) Send([4]byte, []byte) error { return nil }
func (stubDongle) Receive([]byte) (int, bool) { return 0, false }
func (stubDongle) ReplyGuard() time.Duration  { return time.Millisecond }
func (stubDongle) Close() error               { return nil }

// withFakeDongleType swaps dongleTypes for a single fake type whose discover
// returns the given selectors, restoring the original afterwards.
func withFakeDongleType(t *testing.T, selectors []string) {
	t.Helper()
	saved := dongleTypes
	t.Cleanup(func() { dongleTypes = saved })
	dongleTypes = []dongleType{{
		scheme:   "fake",
		discover: func() ([]string, error) { return append([]string(nil), selectors...), nil },
		open: func(string, uint8, config.SpreadFactor, [node.AddrLen]byte) (radio.Dongle, error) {
			return stubDongle{}, nil
		},
		guard: func(config.SpreadFactor) time.Duration { return time.Millisecond },
	}}
}

func mustAddr(t *testing.T, s string) [node.AddrLen]byte {
	t.Helper()
	a, err := node.ParseAddress(s)
	if err != nil {
		t.Fatalf("ParseAddress(%q): %v", s, err)
	}
	return a
}

// TestStartDonglesAlwaysStarts verifies the hub comes up even with no dongle
// connected: startDongles registers one radio per channel and never errors on an
// empty pool.
func TestStartDonglesAlwaysStarts(t *testing.T) {
	withFakeDongleType(t, nil) // no dongles connected

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	eng := engine.New(engine.Options{
		HubAddr: mustAddr(t, "FFFFFF01"), Timeout: time.Second, Retries: 1, RefreshInterval: time.Second,
	})

	chNames := map[uint8]string{37: "far", 38: "near"}
	sfByChannel := map[uint8]config.SpreadFactor{37: config.SpreadFactorS8, 38: config.SpreadFactorS2}

	diag, err := startDongles(ctx, eng, mustAddr(t, "FFFFFF01"), sfByChannel, chNames, slog.Default())
	if err != nil {
		t.Fatalf("startDongles with no dongles: %v", err)
	}
	if len(diag) != 2 {
		t.Fatalf("got %d diag entries, want 2 (one per channel)", len(diag))
	}
	// In ascending channel order, each labelled by its channel name.
	if diag[0].Name != "far" || diag[1].Name != "near" {
		t.Fatalf("diag names = %q, %q; want far, near", diag[0].Name, diag[1].Name)
	}
}

// TestDongleAssignerClaims verifies the assigner lends each connected dongle to
// at most one channel and releases it for reassignment on close.
func TestDongleAssignerClaims(t *testing.T) {
	withFakeDongleType(t, []string{"B", "A"}) // two interchangeable dongles
	a := newDongleAssigner(dongleTypes)
	hub := mustAddr(t, "FFFFFF01")

	// First claim takes the lowest-sorted free selector.
	d1, err := a.claim(37, config.SpreadFactorS8, hub)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	// Second claim takes the other one; the first is still held.
	d2, err := a.claim(38, config.SpreadFactorS2, hub)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	// No dongle left: a third channel gets errNoFreeDongle, not a double-assign.
	if _, err := a.claim(39, config.SpreadFactorS8, hub); err != errNoFreeDongle {
		t.Fatalf("third claim: got %v, want errNoFreeDongle", err)
	}
	// Closing one releases it back to the pool for reassignment.
	d1.Close()
	d3, err := a.claim(39, config.SpreadFactorS8, hub)
	if err != nil {
		t.Fatalf("claim after release: %v", err)
	}
	d2.Close()
	d3.Close()
}

// TestDongleAssignerEmpty verifies an empty pool yields errNoFreeDongle (the
// signal the supervisor retries on), not a hard failure.
func TestDongleAssignerEmpty(t *testing.T) {
	withFakeDongleType(t, nil)
	a := newDongleAssigner(dongleTypes)
	if _, err := a.claim(37, config.SpreadFactorS8, mustAddr(t, "FFFFFF01")); err != errNoFreeDongle {
		t.Fatalf("claim on empty pool: got %v, want errNoFreeDongle", err)
	}
}

func TestSortedChannels(t *testing.T) {
	chs := sortedChannels(map[uint8]string{38: "near", 5: "lo", 37: "far"})
	want := []uint8{5, 37, 38}
	if len(chs) != len(want) {
		t.Fatalf("got %v, want %v", chs, want)
	}
	for i := range want {
		if chs[i] != want[i] {
			t.Fatalf("got %v, want %v", chs, want)
		}
	}
}
