// Package inventory is the type-safe, code-as-configuration model of a BleRiot
// deployment. A site repository declares an Inventory (a slice of Instances) in
// Go and passes it to the host runtime; there is no JSON or external config.
//
// An Instance binds one physical device to:
//   - its identity: the MCU unique ID (from which the RF address is derived),
//     the XTEA key, and the RF channel;
//   - its device type (a shared DeviceType describing the register table);
//   - its device-type-specific Config (a fixed-size struct, see pkg/page).
//
// Register identity on the wire is a stable, hand-assigned Tag (like a protobuf
// field number), not the slice position, so the register table can be reordered
// or extended over time without breaking deployed firmware. Tags are unique and
// non-zero within a device type and must never be reused once retired.
package inventory

import (
	"fmt"

	"cli/pkg/page"
)

// RegType is the hub-side interpretation of a register's int32 wire value.
type RegType string

const (
	TypeInt   RegType = "int"
	TypeFloat RegType = "float"
	TypeBool  RegType = "bool"
)

// Register describes one register of a device type.
type Register struct {
	// Tag is the permanent wire identity of this register: unique and non-zero
	// within its DeviceType, never reused once retired. It is hand-assigned, like
	// a protobuf field number.
	Tag uint8
	// Name is the register name exposed to the hub and Registry (e.g. "setpoint").
	Name string
	// Type interprets the int32 wire value (int/float/bool).
	Type RegType
	// Multiplier and Divider scale the wire value for display:
	// display = wire * Multiplier / Divider. Divider must be non-zero for
	// non-bool registers.
	Multiplier int32
	Divider    int32
	// Metadata is arbitrary descriptive data forwarded to the Registry.
	Metadata map[string]string
}

// DeviceType describes a class of device (e.g. "thermostat"). It is a shared,
// per-type artifact authored once in the device-type module and referenced by
// every Instance of that type.
type DeviceType struct {
	// Name is a stable, human-readable type name.
	Name string
	// Registers is the device's register table. Slice order is irrelevant to the
	// wire; each register is identified by its Tag.
	Registers []Register
}

// Instance is one physical device in the deployment.
type Instance struct {
	// Name uniquely identifies this device within the inventory; it scopes the
	// device's register names in the Registry (e.g. "kitchen").
	Name string
	// UID is the MCU unique ID; the RF address is derived from it.
	UID [page.UIDLen]byte
	// Key is the device's XTEA shared key.
	Key [page.KeyLen]byte
	// Channel is the device's RF channel.
	Channel uint8
	// Type is the device's type (register table).
	Type DeviceType
	// Config is the device-type-specific configuration written to the device's
	// provisioning page. It must be a fixed-size value (see pkg/page); nil means
	// no config.
	Config any
}

// Inventory is the full set of devices in a deployment.
type Inventory []Instance

// Validate checks the device type's register table: Tags must be non-zero and
// unique, register names must be non-empty and unique, and non-bool registers
// must have a non-zero Divider.
func (dt DeviceType) Validate() error {
	if dt.Name == "" {
		return fmt.Errorf("device type: name is required")
	}
	seenTag := make(map[uint8]bool, len(dt.Registers))
	seenName := make(map[string]bool, len(dt.Registers))
	for _, r := range dt.Registers {
		if r.Tag == 0 {
			return fmt.Errorf("device type %q: register %q has reserved tag 0", dt.Name, r.Name)
		}
		if seenTag[r.Tag] {
			return fmt.Errorf("device type %q: duplicate register tag %d", dt.Name, r.Tag)
		}
		seenTag[r.Tag] = true
		if r.Name == "" {
			return fmt.Errorf("device type %q: register with tag %d has no name", dt.Name, r.Tag)
		}
		if seenName[r.Name] {
			return fmt.Errorf("device type %q: duplicate register name %q", dt.Name, r.Name)
		}
		seenName[r.Name] = true
		if r.Type != TypeBool && r.Divider == 0 {
			return fmt.Errorf("device type %q: register %q is non-bool but has zero divider", dt.Name, r.Name)
		}
	}
	return nil
}

// Validate checks the whole inventory: every device type's register table is
// valid and every instance name is non-empty and unique.
func (inv Inventory) Validate() error {
	seenName := make(map[string]bool, len(inv))
	for i, inst := range inv {
		if inst.Name == "" {
			return fmt.Errorf("instance %d: name is required", i)
		}
		if seenName[inst.Name] {
			return fmt.Errorf("duplicate instance name %q", inst.Name)
		}
		seenName[inst.Name] = true
		if err := inst.Type.Validate(); err != nil {
			return fmt.Errorf("instance %q: %w", inst.Name, err)
		}
	}
	return nil
}
