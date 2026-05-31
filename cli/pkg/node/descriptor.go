// Package node models a BleRiot node on the host side: its generated descriptor
// (loaded from the generator's JSON, PROTOCOL.md §11.7) plus its separately
// provisioned identity (address and XTEA key, §11.5).
//
// The descriptor maps wire register IDs to names, hub-side types, and scaling.
// The host bridges these to the external Registry using ToValue/FromValue.
package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

// RegType is the hub-side interpretation of a register's int32 wire value (§11.3).
type RegType string

const (
	TypeInt   RegType = "int"
	TypeFloat RegType = "float"
	TypeBool  RegType = "bool"
)

// Register is one resolved register from a node descriptor.
type Register struct {
	ID         uint16            `json:"id"`
	Name       string            `json:"name"`
	Class      string            `json:"class"`
	Instance   string            `json:"instance"`
	Type       RegType           `json:"type"`
	Multiplier int32             `json:"multiplier"`
	Divider    int32             `json:"divider"`
	Metadata   map[string]string `json:"metadata"`
}

// Descriptor is a node's generated register table (§11.7), plus indexes for
// lookup by wire ID and by qualified name. It is a shared, per-type artifact and
// carries no node name or RF channel; those are per-device facts that come from
// the node's instance file (name) and provisioning (channel). The descriptor ID
// is not stored in the file; the file is content-addressed by that ID.
type Descriptor struct {
	Metadata  map[string]string `json:"metadata"`
	Registers []Register        `json:"registers"`

	byID   map[uint16]*Register
	byName map[string]*Register
}

// LoadDescriptor parses a node descriptor from JSON and builds its indexes,
// validating that IDs and names are unique and non-degenerate.
func LoadDescriptor(r io.Reader) (*Descriptor, error) {
	var d Descriptor
	dec := json.NewDecoder(r)
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("decode node descriptor: %w", err)
	}
	if err := d.index(); err != nil {
		return nil, err
	}
	return &d, nil
}

// LoadDescriptorFile loads a node descriptor from a file path.
func LoadDescriptorFile(path string) (*Descriptor, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LoadDescriptor(f)
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
		if r.Type != TypeBool && r.Divider == 0 {
			return fmt.Errorf("register %q has zero divider", r.Name)
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

// ToValue converts a raw int32 wire value into the Registry-facing value for
// this register's type: bool, int64, or float64.
func (r *Register) ToValue(wire int32) any {
	switch r.Type {
	case TypeBool:
		return wire != 0
	case TypeFloat:
		return float64(wire) * float64(r.Multiplier) / float64(r.Divider)
	default: // int
		return int64(wire)
	}
}

// FromValue converts a Registry-facing value into the raw int32 wire value for
// this register's type. Numeric values are accepted as any of Go's int/float
// kinds (as produced by JSON unmarshalling into any).
func (r *Register) FromValue(v any) (int32, error) {
	switch r.Type {
	case TypeBool:
		b, ok := v.(bool)
		if !ok {
			return 0, fmt.Errorf("register %q expects bool, got %T", r.Name, v)
		}
		if b {
			return 1, nil
		}
		return 0, nil
	case TypeFloat:
		f, err := toFloat(v)
		if err != nil {
			return 0, fmt.Errorf("register %q: %w", r.Name, err)
		}
		scaled := f * float64(r.Divider) / float64(r.Multiplier)
		return saturateInt32(math.Round(scaled)), nil
	default: // int
		f, err := toFloat(v)
		if err != nil {
			return 0, fmt.Errorf("register %q: %w", r.Name, err)
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
