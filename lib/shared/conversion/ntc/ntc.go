// Package ntc provides hub-side conversions for negative temperature
// coefficient thermistors. Nodes can report an unprocessed ADC sample while
// the hub performs the floating-point sensor calculation.
//
// For a low-side thermistor, the resistance is calculated as:
//
//	Rntc = Rfixed * raw / (ADCMax - raw)
//
// A high-side thermistor uses the reciprocal divider ratio. Temperature then
// follows the beta equation, with T and T0 in kelvins:
//
//	1/T = 1/T0 + ln(Rntc/R0)/Beta
package ntc

import (
	"fmt"
	"math"

	"github.com/burgrp/bleriot/lib/shared/inventory"
)

// Position identifies which side of a resistive divider contains the
// thermistor.
type Position uint8

const (
	// ThermistorLowSide describes this divider:
	//
	//	Vref -- fixed resistor -- ADC -- thermistor -- ground
	ThermistorLowSide Position = iota
	// ThermistorHighSide describes this divider:
	//
	//	Vref -- thermistor -- ADC -- fixed resistor -- ground
	ThermistorHighSide
)

// BetaParams describes an NTC thermistor and its ADC divider. Resistances are
// in ohms, temperatures are in degrees Celsius, and Beta is in kelvins.
type BetaParams struct {
	// ADCMax is the ADC code at the reference voltage, such as 4095 for a
	// 12-bit ADC. Raw codes 0 and ADCMax are invalid divider endpoints.
	ADCMax int32
	// FixedResistance is the resistance of the non-thermistor divider resistor.
	FixedResistance float64
	// NominalResistance is the thermistor resistance at NominalTemperatureC.
	NominalResistance float64
	// NominalTemperatureC is the temperature at which NominalResistance is
	// specified, commonly 25 degrees Celsius.
	NominalTemperatureC float64
	// Beta is the thermistor's beta coefficient in kelvins.
	Beta float64
	// Position selects whether the thermistor is on the low or high side of the
	// divider. The zero value is ThermistorLowSide.
	Position Position
}

// Beta returns a read-only conversion that maps a raw ADC code to degrees
// Celsius using the thermistor beta equation. Use it on a Register with
// ReadOnly set to true. The returned Conversion intentionally has no Encode
// function because temperature-to-ADC writes do not represent a sensor action.
//
// Decode rejects raw values at or beyond the divider endpoints because they
// imply zero or infinite resistance. Beta panics when params do not describe a
// physically valid divider; malformed constants are a device-specification
// programming error and should fail immediately when Type() is constructed.
func Beta(params BetaParams) inventory.Conversion {
	validate(params)
	nominalKelvin := params.NominalTemperatureC + 273.15
	return inventory.Conversion{
		Decode: func(raw int32) (any, error) {
			if raw <= 0 || raw >= params.ADCMax {
				return nil, fmt.Errorf("ntc: ADC value %d outside valid range 1..%d", raw, params.ADCMax-1)
			}

			ratio := float64(raw) / float64(params.ADCMax-raw)
			if params.Position == ThermistorHighSide {
				ratio = 1 / ratio
			}
			resistance := params.FixedResistance * ratio
			inverseKelvin := 1/nominalKelvin + math.Log(resistance/params.NominalResistance)/params.Beta
			if inverseKelvin <= 0 || math.IsNaN(inverseKelvin) || math.IsInf(inverseKelvin, 0) {
				return nil, fmt.Errorf("ntc: ADC value %d produces an invalid temperature", raw)
			}
			temperatureC := 1/inverseKelvin - 273.15
			if math.IsNaN(temperatureC) || math.IsInf(temperatureC, 0) {
				return nil, fmt.Errorf("ntc: ADC value %d produces an invalid temperature", raw)
			}
			return temperatureC, nil
		},
	}
}

func validate(params BetaParams) {
	if params.ADCMax < 2 {
		panic("ntc: ADCMax must be at least 2")
	}
	requirePositiveFinite("FixedResistance", params.FixedResistance)
	requirePositiveFinite("NominalResistance", params.NominalResistance)
	requirePositiveFinite("Beta", params.Beta)
	if math.IsNaN(params.NominalTemperatureC) || math.IsInf(params.NominalTemperatureC, 0) || params.NominalTemperatureC <= -273.15 {
		panic("ntc: NominalTemperatureC must be finite and above absolute zero")
	}
	if params.Position != ThermistorLowSide && params.Position != ThermistorHighSide {
		panic("ntc: invalid thermistor position")
	}
}

func requirePositiveFinite(name string, value float64) {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		panic(fmt.Sprintf("ntc: %s must be finite and positive", name))
	}
}
