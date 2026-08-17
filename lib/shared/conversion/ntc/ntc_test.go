package ntc

import (
	"math"
	"testing"
)

func testParams() BetaParams {
	return BetaParams{
		ADCMax:              1000,
		FixedResistance:     10_000,
		NominalResistance:   10_000,
		NominalTemperatureC: 25,
		Beta:                3950,
	}
}

func TestBetaNominalTemperature(t *testing.T) {
	for _, position := range []Position{ThermistorLowSide, ThermistorHighSide} {
		params := testParams()
		params.Position = position
		conversion := Beta(params)
		got, err := conversion.Decode(500)
		if err != nil || math.Abs(got.(float64)-25) > 1e-12 {
			t.Errorf("position %d: Decode(500) = %v, %v; want 25, nil", position, got, err)
		}
		if conversion.Encode != nil {
			t.Errorf("position %d: NTC conversion has an encoder", position)
		}
	}
}

func TestBetaKnownTemperature(t *testing.T) {
	params := testParams()
	params.ADCMax = 1_000_000
	conversion := Beta(params)

	// A 10k, beta-3950 thermistor is approximately 697.52 ohms at 100 C.
	resistance := 697.519772
	raw := int32(math.Round(float64(params.ADCMax) * resistance / (params.FixedResistance + resistance)))
	value, err := conversion.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := value.(float64); math.Abs(got-100) > 0.01 {
		t.Fatalf("Decode(%d) = %.6f C, want 100 C", raw, got)
	}
}

func TestBetaDividerOrientation(t *testing.T) {
	lowParams := testParams()
	highParams := testParams()
	highParams.Position = ThermistorHighSide
	low := Beta(lowParams)
	high := Beta(highParams)

	lowValue, err := low.Decode(250)
	if err != nil {
		t.Fatal(err)
	}
	highValue, err := high.Decode(750)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(lowValue.(float64)-highValue.(float64)) > 1e-12 {
		t.Fatalf("equivalent divider samples differ: low %.9f C, high %.9f C", lowValue, highValue)
	}
}

func TestBetaRejectsDividerEndpoints(t *testing.T) {
	conversion := Beta(testParams())
	for _, raw := range []int32{-1, 0, 1000, 1001} {
		if _, err := conversion.Decode(raw); err == nil {
			t.Errorf("Decode(%d) succeeded, want error", raw)
		}
	}
}

func TestBetaRejectsInvalidParameters(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*BetaParams)
	}{
		{name: "ADC maximum", mutate: func(params *BetaParams) { params.ADCMax = 1 }},
		{name: "fixed resistance", mutate: func(params *BetaParams) { params.FixedResistance = 0 }},
		{name: "nominal resistance", mutate: func(params *BetaParams) { params.NominalResistance = math.Inf(1) }},
		{name: "nominal temperature", mutate: func(params *BetaParams) { params.NominalTemperatureC = -273.15 }},
		{name: "beta", mutate: func(params *BetaParams) { params.Beta = math.NaN() }},
		{name: "position", mutate: func(params *BetaParams) { params.Position = Position(2) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			params := testParams()
			test.mutate(&params)
			defer func() {
				if recover() == nil {
					t.Fatal("Beta did not panic")
				}
			}()
			Beta(params)
		})
	}
}
