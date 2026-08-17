package conversion

import (
	"encoding/json"
	"math"
	"testing"
)

func TestScale(t *testing.T) {
	conversion := Scale(0.01)
	got, err := conversion.Decode(1234)
	if err != nil || got != 12.34 {
		t.Fatalf("Decode(1234) = %v, %v; want 12.34, nil", got, err)
	}

	for _, test := range []struct {
		name  string
		value any
		want  int32
	}{
		{name: "float64", value: 12.34, want: 1234},
		{name: "float32", value: float32(12.34), want: 1234},
		{name: "int", value: 12, want: 1200},
		{name: "int64", value: int64(12), want: 1200},
		{name: "uint16", value: uint16(12), want: 1200},
		{name: "json number", value: json.Number("12.34"), want: 1234},
		{name: "round down", value: 12.344, want: 1234},
		{name: "round up", value: 12.345, want: 1235},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := conversion.Encode(test.value)
			if err != nil || got != test.want {
				t.Fatalf("Encode(%v) = %d, %v; want %d, nil", test.value, got, err, test.want)
			}
		})
	}
}

func TestLinear(t *testing.T) {
	conversion := Linear(-0.5, 10)
	got, err := conversion.Decode(8)
	if err != nil || got != 6.0 {
		t.Fatalf("Decode(8) = %v, %v; want 6, nil", got, err)
	}
	raw, err := conversion.Encode(6.0)
	if err != nil || raw != 8 {
		t.Fatalf("Encode(6) = %d, %v; want 8, nil", raw, err)
	}
}

func TestLinearRoundTrip(t *testing.T) {
	conversion := Linear(0.125, -40)
	for _, raw := range []int32{math.MinInt32, -12345, -1, 0, 1, 12345, math.MaxInt32} {
		value, err := conversion.Decode(raw)
		if err != nil {
			t.Fatalf("Decode(%d): %v", raw, err)
		}
		got, err := conversion.Encode(value)
		if err != nil || got != raw {
			t.Errorf("round trip %d -> %v -> %d, %v", raw, value, got, err)
		}
	}
}

func TestLinearSaturates(t *testing.T) {
	conversion := Scale(1)
	if got, err := conversion.Encode(float64(math.MaxInt32) * 2); err != nil || got != math.MaxInt32 {
		t.Fatalf("positive overflow = %d, %v; want %d, nil", got, err, math.MaxInt32)
	}
	if got, err := conversion.Encode(float64(math.MinInt32) * 2); err != nil || got != math.MinInt32 {
		t.Fatalf("negative overflow = %d, %v; want %d, nil", got, err, math.MinInt32)
	}
}

func TestLinearRejectsNonNumericValues(t *testing.T) {
	conversion := Scale(1)
	for _, value := range []any{"12", true, nil, math.NaN(), math.Inf(1), json.Number("bad")} {
		if _, err := conversion.Encode(value); err == nil {
			t.Errorf("Encode(%v) succeeded, want error", value)
		}
	}
}

func TestLinearRejectsNonFiniteDecodedValue(t *testing.T) {
	conversion := Linear(math.MaxFloat64, math.MaxFloat64)
	if _, err := conversion.Decode(2); err == nil {
		t.Fatal("Decode produced an infinite value without an error")
	}
}

func TestLinearRejectsInvalidParameters(t *testing.T) {
	for _, test := range []struct {
		name   string
		factor float64
		offset float64
	}{
		{name: "zero factor", factor: 0},
		{name: "NaN factor", factor: math.NaN()},
		{name: "infinite factor", factor: math.Inf(1)},
		{name: "NaN offset", factor: 1, offset: math.NaN()},
		{name: "infinite offset", factor: 1, offset: math.Inf(-1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("Linear did not panic")
				}
			}()
			Linear(test.factor, test.offset)
		})
	}
}
