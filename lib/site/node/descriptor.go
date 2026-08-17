// Package node models a BleRiot node on the host side: its register descriptor
// (built from a device type's register table) plus its separately provisioned
// identity (address and XTEA key, lib/README.md §11.5).
//
// The descriptor maps wire register IDs to names, hub-side types, and value
// conversion functions. The host bridges these to the external Registry.
package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

// RegType is the hub-side interpretation of a register's int32 wire value (§11.3).
type RegType string

const (
	TypeInt   RegType = "int"
	TypeFloat RegType = "float"
	TypeBool  RegType = "bool"
)

// Conversion translates between a raw BleRiot wire value and the typed value
// exposed through the Registry. Decode is always populated by NewDescriptor.
// Encode is nil exactly when the register is read-only.
type Conversion struct {
	Decode func(raw int32) (any, error)
	Encode func(value any) (int32, error)
}

// Register is one resolved register from a node descriptor.
type Register struct {
	ID       uint16
	Name     string
	Type     RegType
	ReadOnly bool
	// Conversion translates raw wire values to and from Registry-facing values.
	// NewDescriptor installs Type-based defaults for omitted functions.
	Conversion Conversion
	Metadata   map[string]string
}

// Descriptor is a node's register table, plus indexes for lookup by wire ID and
// by qualified name. It is a shared, per-type artifact and carries no node name
// or RF channel; those are per-device facts that come from the inventory
// instance (name) and provisioning (channel).
type Descriptor struct {
	Metadata  map[string]string
	Registers []Register

	byID   map[uint16]*Register
	byName map[string]*Register
}

// NewDescriptor builds a Descriptor from an in-memory register table (e.g. one
// derived from an inventory device type) and validates it: register IDs and
// names must be unique and non-degenerate.
func NewDescriptor(metadata map[string]string, regs []Register) (*Descriptor, error) {
	d := &Descriptor{Metadata: metadata, Registers: regs}
	if err := d.index(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Descriptor) index() error {
	d.byID = make(map[uint16]*Register, len(d.Registers))
	d.byName = make(map[string]*Register, len(d.Registers))
	for i := range d.Registers {
		r := &d.Registers[i]
		if r.ID == 0 {
			return fmt.Errorf("register %q has reserved ID 0x0000", r.Name)
		}
		if _, dup := d.byID[r.ID]; dup {
			return fmt.Errorf("duplicate register ID 0x%04X", r.ID)
		}
		if _, dup := d.byName[r.Name]; dup {
			return fmt.Errorf("duplicate register name %q", r.Name)
		}
		if r.ReadOnly {
			if r.Conversion.Encode != nil {
				return fmt.Errorf("read-only register %q must not define Conversion.Encode", r.Name)
			}
			if r.Conversion.Decode == nil {
				r.Conversion.Decode = defaultConversion(r.Name, r.Type).Decode
			}
		} else {
			if (r.Conversion.Decode == nil) != (r.Conversion.Encode == nil) {
				return fmt.Errorf("writable register %q must define both Conversion.Decode and Conversion.Encode", r.Name)
			}
			if r.Conversion.Decode == nil {
				r.Conversion = defaultConversion(r.Name, r.Type)
			}
		}
		d.byID[r.ID] = r
		d.byName[r.Name] = r
	}
	return nil
}

// ByID returns the register with the given wire ID.
func (d *Descriptor) ByID(id uint16) (*Register, bool) {
	r, ok := d.byID[id]
	return r, ok
}

// ByName returns the register with the given qualified name.
func (d *Descriptor) ByName(name string) (*Register, bool) {
	r, ok := d.byName[name]
	return r, ok
}

// defaultConversion returns the Registry conversion implied by a register's
// Type. Numeric Registry values are accepted in Go's common int/float forms.
func defaultConversion(name string, typ RegType) Conversion {
	switch typ {
	case TypeBool:
		return Conversion{
			Decode: func(raw int32) (any, error) { return raw != 0, nil },
			Encode: func(value any) (int32, error) {
				b, ok := value.(bool)
				if !ok {
					return 0, fmt.Errorf("register %q expects bool, got %T", name, value)
				}
				if b {
					return 1, nil
				}
				return 0, nil
			},
		}
	case TypeFloat:
		return Conversion{
			Decode: func(raw int32) (any, error) { return float64(raw), nil },
			Encode: numericEncode(name),
		}
	default: // int
		return Conversion{
			Decode: func(raw int32) (any, error) { return int64(raw), nil },
			Encode: numericEncode(name),
		}
	}
}

func numericEncode(name string) func(any) (int32, error) {
	return func(value any) (int32, error) {
		f, err := toFloat(value)
		if err != nil {
			return 0, fmt.Errorf("register %q: %w", name, err)
		}
		return saturateInt32(math.Round(f)), nil
	}
}

func toFloat(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case json.Number:
		return n.Float64()
	default:
		return 0, errors.New("expected a numeric value")
	}
}

func saturateInt32(f float64) int32 {
	if f >= math.MaxInt32 {
		return math.MaxInt32
	}
	if f <= math.MinInt32 {
		return math.MinInt32
	}
	return int32(f)
}
