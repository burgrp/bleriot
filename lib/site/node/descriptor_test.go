package node

import (
	"math"
	"testing"
)

func scaledTemperatureDecode(raw int32) (any, error) {
	return float64(raw) / 100, nil
}

func scaledTemperatureEncode(value any) (int32, error) {
	f, err := toFloat(value)
	if err != nil {
		return 0, err
	}
	return saturateInt32(math.Round(f * 100)), nil
}

func sampleRegisters() []Register {
	return []Register{
		{ID: 7911, Name: "outdoor.temperature", Type: TypeFloat, Conversion: Conversion{Decode: scaledTemperatureDecode, Encode: scaledTemperatureEncode}, Metadata: map[string]string{"unit": "celsius"}},
		{ID: 6470, Name: "outdoor.humidity", Type: TypeInt},
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

func TestDecode(t *testing.T) {
	d := loadSample(t)

	temp, _ := d.ByID(7911) // float, /100
	if got, err := temp.Conversion.Decode(1234); err != nil || got != 12.34 {
		t.Errorf("temperature Decode(1234) = %v, %v; want 12.34, nil", got, err)
	}
	hum, _ := d.ByID(6470) // int
	if got, err := hum.Conversion.Decode(57); err != nil || got != int64(57) {
		t.Errorf("humidity Decode(57) = %v, %v; want 57, nil", got, err)
	}
	relay, _ := d.ByID(4466) // bool
	if got, err := relay.Conversion.Decode(0); err != nil || got != false {
		t.Errorf("relay Decode(0) = %v, %v; want false, nil", got, err)
	}
	if got, err := relay.Conversion.Decode(1); err != nil || got != true {
		t.Errorf("relay Decode(1) = %v, %v; want true, nil", got, err)
	}
}

func TestEncode(t *testing.T) {
	d := loadSample(t)

	temp, _ := d.ByID(7911) // float, /100
	w, err := temp.Conversion.Encode(12.34)
	if err != nil || w != 1234 {
		t.Errorf("temperature Encode(12.34) = %d, %v; want 1234", w, err)
	}
	// JSON numbers arrive as float64.
	hum, _ := d.ByID(6470)
	w, err = hum.Conversion.Encode(float64(57))
	if err != nil || w != 57 {
		t.Errorf("humidity Encode(57.0) = %d, %v; want 57", w, err)
	}
	relay, _ := d.ByID(4466)
	if w, _ := relay.Conversion.Encode(true); w != 1 {
		t.Errorf("relay Encode(true) = %d, want 1", w)
	}
	if w, _ := relay.Conversion.Encode(false); w != 0 {
		t.Errorf("relay Encode(false) = %d, want 0", w)
	}
	if _, err := relay.Conversion.Encode(42.0); err == nil {
		t.Error("relay Encode(number) should error (expects bool)")
	}
	if _, err := temp.Conversion.Encode("nope"); err == nil {
		t.Error("temperature Encode(string) should error")
	}
}

func TestFloatRoundTrip(t *testing.T) {
	d := loadSample(t)
	temp, _ := d.ByID(7911)
	for _, wire := range []int32{0, 1, 1234, -2500, 32000} {
		v, err := temp.Conversion.Decode(wire)
		if err != nil {
			t.Fatalf("Decode(%d): %v", wire, err)
		}
		back, err := temp.Conversion.Encode(v)
		if err != nil || back != wire {
			t.Errorf("round trip %d -> %v -> %d (err %v)", wire, v, back, err)
		}
	}
}

func TestDefaultFloatConversion(t *testing.T) {
	d, err := NewDescriptor(nil, []Register{{ID: 1, Name: "value", Type: TypeFloat}})
	if err != nil {
		t.Fatal(err)
	}
	r, _ := d.ByID(1)
	if got, err := r.Conversion.Decode(42); err != nil || got != float64(42) {
		t.Fatalf("Decode(42) = %v, %v; want float64(42), nil", got, err)
	}
	if got, err := r.Conversion.Encode(42.4); err != nil || got != 42 {
		t.Fatalf("Encode(42.4) = %d, %v; want 42, nil", got, err)
	}
}

func TestReadOnlyConversion(t *testing.T) {
	d, err := NewDescriptor(nil, []Register{{
		ID:       1,
		Name:     "temperature",
		Type:     TypeFloat,
		ReadOnly: true,
		Conversion: Conversion{Decode: func(raw int32) (any, error) {
			return float64(raw) / 10, nil
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	r, _ := d.ByID(1)
	if got, err := r.Conversion.Decode(253); err != nil || got != 25.3 {
		t.Fatalf("Decode(253) = %v, %v; want 25.3, nil", got, err)
	}
	if r.Conversion.Encode != nil {
		t.Fatal("read-only conversion has an encoder")
	}
}
