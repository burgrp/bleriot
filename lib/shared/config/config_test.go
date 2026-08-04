package config

import "testing"

// TestSpreadFactorString checks the symbolic spread-factor names used in logs.
func TestSpreadFactorString(t *testing.T) {
	cases := map[SpreadFactor]string{
		SpreadFactorS8:     "S8",
		SpreadFactorS2:     "S2",
		SpreadFactor(0xFF): "S?",
	}
	for sf, want := range cases {
		if got := sf.String(); got != want {
			t.Errorf("SpreadFactor(%d).String() = %q, want %q", uint8(sf), got, want)
		}
	}
}
