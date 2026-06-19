package host

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"cli/pkg/inventory"
	"cli/pkg/node"
	"cli/pkg/page"
)

// fakeProbe is an in-memory Probe for tests.
type fakeProbe struct {
	uid      [page.UIDLen]byte
	readErr  error
	written  []byte
	writeErr error
}

func (f *fakeProbe) ReadUID(context.Context) ([page.UIDLen]byte, error) {
	return f.uid, f.readErr
}

func (f *fakeProbe) WritePage(_ context.Context, image []byte) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.written = append([]byte(nil), image...)
	return nil
}

type thermostatConfig struct {
	MinTemp int16
	MaxTemp int16
}

func sampleType() inventory.DeviceType {
	return inventory.DeviceType{
		Name: "thermostat",
		Chip: inventory.PY32F030,
		Registers: []inventory.Register{
			{Tag: 1, Name: "temperature", Type: inventory.TypeFloat, Multiplier: 1, Divider: 100},
			{Tag: 2, Name: "heating", Type: inventory.TypeBool},
		},
	}
}

func sampleInstance() inventory.Instance {
	return inventory.Instance{
		Name:    "kitchen",
		UID:     [page.UIDLen]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
		Key:     [page.KeyLen]byte{0xAA, 0xBB, 0xCC, 0xDD},
		Channel: inventory.Channel{Number: 37, SpreadFactor: page.SpreadFactorS2},
		Type:    sampleType(),
		Config:  thermostatConfig{MinTemp: 1800, MaxTemp: 2400},
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
	if r, ok := n.ByID(1); !ok || r.Name != "temperature" {
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

	if err := runProvision(context.Background(), inv, inventory.PY32F030, fp, discardLogger()); err != nil {
		t.Fatalf("runProvision: %v", err)
	}
	if fp.written == nil {
		t.Fatal("no page written")
	}

	var got thermostatConfig
	h, err := page.Unmarshal(fp.written, &got)
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
	fp := &fakeProbe{uid: [page.UIDLen]byte{0xFF}} // not in inventory

	err := runProvision(context.Background(), inv, inventory.PY32F030, fp, discardLogger())
	if err == nil {
		t.Fatal("expected error for unknown UID")
	}
	if fp.written != nil {
		t.Fatal("must not write a page for an unknown device")
	}
}

func TestRunNewPrintsStub(t *testing.T) {
	inv := inventory.Inventory{sampleInstance()}
	uid := [page.UIDLen]byte{0x10, 0x20, 0x30}
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

func TestParseDongles(t *testing.T) {
	specs, err := parseDongles([]string{"mcp2210:/dev/hidraw0,37", "mcp2210:ABC123,11", "mcp2210:/dev/hidraw1,5"})
	if err != nil {
		t.Fatalf("parseDongles: %v", err)
	}
	if len(specs) != 3 {
		t.Fatalf("got %d specs, want 3", len(specs))
	}
	if specs[0].scheme != "mcp2210" || specs[0].selector != "/dev/hidraw0" || specs[0].channel != 37 {
		t.Fatalf("spec[0] = %+v, want mcp2210 /dev/hidraw0 ch 37", specs[0])
	}
	if specs[1].scheme != "mcp2210" || specs[1].selector != "ABC123" || specs[1].channel != 11 {
		t.Fatalf("spec[1] = %+v, want mcp2210 ABC123 ch 11", specs[1])
	}
	if specs[2].scheme != "mcp2210" || specs[2].selector != "/dev/hidraw1" || specs[2].channel != 5 {
		t.Fatalf("spec[2] = %+v, want mcp2210 /dev/hidraw1 ch 5", specs[2])
	}
}

func TestParseDonglesErrors(t *testing.T) {
	cases := []string{
		"mcp2210:/dev/hidraw0",     // missing channel
		"mcp2210:,37",              // empty selector
		"mcp2210:/dev/hidraw0,x",   // non-numeric channel
		"mcp2210:/dev/hidraw0,300", // channel out of uint8 range
		"bogus:ABC123,37",          // unknown dongle type
		"ABC123,37",                // missing scheme prefix
	}
	for _, c := range cases {
		if _, err := parseDongles([]string{c}); err == nil {
			t.Fatalf("parseDongles(%q): expected error", c)
		}
	}
}
