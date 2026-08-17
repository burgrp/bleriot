// Package conversion provides reusable hub-side conversions for BleRiot
// register specifications. The node still reads and writes raw int32 values;
// these functions execute only in the hub's Registry bridge.
package conversion

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/burgrp/bleriot/lib/shared/inventory"
)

// Scale returns a conversion that exposes raw*factor through the Registry.
// Encoding applies the inverse operation and rounds to the nearest wire value,
// saturating results outside the int32 range. It accepts the common Go numeric
// types and json.Number.
//
// Scale panics if factor is zero, NaN, or infinite. Those values cannot define
// an invertible writable conversion and indicate an invalid device
// specification.
func Scale(factor float64) inventory.Conversion {
	return Linear(factor, 0)
}

// Linear returns a conversion that exposes raw*factor+offset through the
// Registry. Encoding applies the inverse operation, rounds to the nearest wire
// value, and saturates results outside the int32 range. It accepts the common Go
// numeric types and json.Number.
//
// Linear panics if factor is zero, NaN, or infinite, or if offset is NaN or
// infinite. Such values indicate an invalid device specification.
func Linear(factor, offset float64) inventory.Conversion {
	if factor == 0 || math.IsNaN(factor) || math.IsInf(factor, 0) {
		panic("conversion: factor must be finite and non-zero")
	}
	if math.IsNaN(offset) || math.IsInf(offset, 0) {
		panic("conversion: offset must be finite")
	}
	return inventory.Conversion{
		Decode: func(raw int32) (any, error) {
			value := float64(raw)*factor + offset
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, fmt.Errorf("conversion: raw value %d produces a non-finite value", raw)
			}
			return value, nil
		},
		Encode: func(value any) (int32, error) {
			number, err := numericValue(value)
			if err != nil {
				return 0, err
			}
			return saturateInt32(math.Round((number - offset) / factor)), nil
		},
	}
}

func numericValue(value any) (float64, error) {
	var number float64
	switch value := value.(type) {
	case float64:
		number = value
	case float32:
		number = float64(value)
	case int:
		number = float64(value)
	case int8:
		number = float64(value)
	case int16:
		number = float64(value)
	case int32:
		number = float64(value)
	case int64:
		number = float64(value)
	case uint:
		number = float64(value)
	case uint8:
		number = float64(value)
	case uint16:
		number = float64(value)
	case uint32:
		number = float64(value)
	case uint64:
		number = float64(value)
	case json.Number:
		var err error
		number, err = value.Float64()
		if err != nil {
			return 0, fmt.Errorf("conversion: expected a numeric value: %w", err)
		}
	default:
		return 0, fmt.Errorf("conversion: expected a numeric value, got %T", value)
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, fmt.Errorf("conversion: numeric value must be finite")
	}
	return number, nil
}

func saturateInt32(value float64) int32 {
	if value >= math.MaxInt32 {
		return math.MaxInt32
	}
	if value <= math.MinInt32 {
		return math.MinInt32
	}
	return int32(value)
}
