package node

import (
	"testing"
)

func sampleRegisters() []Register {
	return []Register{
		{ID: 7911, Name: "outdoor.temperature", Type: TypeFloat, Multiplier: 1, Divider: 100, Metadata: map[string]string{"unit": "celsius"}},
		{ID: 6470, Name: "outdoor.humidity", Type: TypeInt, Multiplier: 1, Divider: 1},
		{ID: 4466, Name: "aux.relay", Type: TypeBool},
	}
}

func loadSample(t *testing.T) *Descriptor {
	t.Helper()
	d, err := NewDescriptor(map[string]string{"hw_rev": "1.3"}, sampleRegisters())
	if err != nil {
		t.Fatalf("NewDescriptor: %v", err)
	}
	return d
}

func TestDescriptor_Indexes(t *testing.T) {
	d := loadSample(t)
	if len(d.Registers) != 3 {
		t.Fatalf("got %d registers, want 3", len(d.Registers))
	}
	r, ok := d.ByID(7911)
	if !ok || r.Name != "outdoor.temperature" {
		t.Errorf("ByID(7911) = %+v, %v", r, ok)
	}
	r, ok = d.ByName("aux.relay")
	if !ok || r.ID != 4466 {
		t.Errorf("ByName(aux.relay) = %+v, %v", r, ok)
	}
	if _, ok := d.ByID(0xFFFF); ok {
		t.Error("ByID of missing register should fail")
	}
}

func TestToValue(t *testing.T) {
	d := loadSample(t)

	temp, _ := d.ByID(7911) // float, /100
	if got := temp.ToValue(1234); got != 12.34 {
		t.Errorf("temperature ToValue(1234) = %v, want 12.34", got)
	}
	hum, _ := d.ByID(6470) // int
	if got := hum.ToValue(57); got != int64(57) {
		t.Errorf("humidity ToValue(57) = %v, want 57", got)
	}
	relay, _ := d.ByID(4466) // bool
	if got := relay.ToValue(0); got != false {
		t.Errorf("relay ToValue(0) = %v, want false", got)
	}
	if got := relay.ToValue(1); got != true {
		t.Errorf("relay ToValue(1) = %v, want true", got)
	}
}

func TestFromValue(t *testing.T) {
	d := loadSample(t)

	temp, _ := d.ByID(7911) // float, /100
	w, err := temp.FromValue(12.34)
	if err != nil || w != 1234 {
		t.Errorf("temperature FromValue(12.34) = %d, %v; want 1234", w, err)
	}
	// JSON numbers arrive as float64.
	hum, _ := d.ByID(6470)
	w, err = hum.FromValue(float64(57))
	if err != nil || w != 57 {
		t.Errorf("humidity FromValue(57.0) = %d, %v; want 57", w, err)
	}
	relay, _ := d.ByID(4466)
	if w, _ := relay.FromValue(true); w != 1 {
		t.Errorf("relay FromValue(true) = %d, want 1", w)
	}
	if w, _ := relay.FromValue(false); w != 0 {
		t.Errorf("relay FromValue(false) = %d, want 0", w)
	}
	if _, err := relay.FromValue(42.0); err == nil {
		t.Error("relay FromValue(number) should error (expects bool)")
	}
	if _, err := temp.FromValue("nope"); err == nil {
		t.Error("temperature FromValue(string) should error")
	}
}

func TestFloatRoundTrip(t *testing.T) {
	d := loadSample(t)
	temp, _ := d.ByID(7911)
	for _, wire := range []int32{0, 1, 1234, -2500, 32000} {
		v := temp.ToValue(wire)
		back, err := temp.FromValue(v)
		if err != nil || back != wire {
			t.Errorf("round trip %d -> %v -> %d (err %v)", wire, v, back, err)
		}
	}
}
