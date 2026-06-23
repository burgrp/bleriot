// Package inventory is the type-safe, code-as-configuration model of a BleRiot
// deployment. A site repository declares an Inventory (a slice of Instances) in
// Go and passes it to the host runtime; there is no JSON or external config.
//
// An Instance binds one physical device to:
//   - its identity: the MCU unique ID (from which the RF address is derived),
//     the XTEA key, and the RF channel;
//   - its device type (a shared DeviceType describing the register table);
//   - its device-type-specific Config (a fixed-size struct, see pkg/config).
//
// Register identity on the wire is a stable, hand-assigned Tag (like a protobuf
// field number), not the slice position, so the register table can be reordered
// or extended over time without breaking deployed firmware. Tags are unique and
// non-zero within a device type and must never be reused once retired.
package inventory

import (
	"fmt"

	"github.com/burgrp/bleriot/site/config"
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

// DeviceType describes a class of device (e.g. "bob"). It is a shared,
// per-type artifact authored once in the device-type module and referenced by
// every Instance of that type.
type DeviceType struct {
	// Name is a stable, human-readable type name.
	Name string
	// Registers is the device's register table. Slice order is irrelevant to the
	// wire; each register is identified by its Tag.
	Registers []Register
	// Chip is the MCU the device's firmware runs on. It tells the provisioning
	// commands how to reach the device over SWD (pyocd target, UID and page
	// addresses). It is only needed for provisioning; the hub ignores it.
	Chip Chip
}

// Chip describes an MCU type from the provisioning tooling's point of view: how
// to address it over SWD. It is a firmware/hardware fact of a device type, not a
// per-device deployment fact, so it lives on the DeviceType. Predefined chips
// are provided as package-level values (e.g. PY32F030); a site can also declare
// its own.
type Chip struct {
	// Name selects the chip on the command line (--chip) and identifies it in
	// errors, e.g. "py32f030".
	Name string
	// Target is the pyocd target name, e.g. "py32f030x8".
	Target string
	// UIDAddr is the memory address of the 12-byte MCU unique ID.
	UIDAddr uint32
	// PageAddr is the flash address of the provisioning page.
	PageAddr uint32
	// PageBytes is the size of the read window the firmware maps at PageAddr on
	// boot: large enough for the provisioning header, the device Config, and the
	// trailing CRC. The decoder tolerates slack, so it need not be exact.
	PageBytes uint32
}

// PY32F030 is the Puya PY32F030 chip profile.
var PY32F030 = Chip{
	Name:      "py32f030",
	Target:    "py32f030x8",
	UIDAddr:   0x1FFF0E00,
	PageAddr:  0x0800F800,
	PageBytes: 64,
}

// Channel is an RF channel together with the spreading factor every node on it
// uses. Spreading factor is a property of the channel as driven by a dongle (a
// dongle transmits one factor at a time), not of an individual node, so binding
// the two in a single value makes it impossible to give two nodes on the same
// channel different factors. Declare each channel once and share that value
// across the instances that use it.
type Channel struct {
	// Name is the channel's human-readable identity, e.g. "far". It is required
	// and must be unique: the hub uses it to scope per-dongle diagnostic registers
	// (e.g. "diag.dongle.far.connected"), so two channels may not share a name and
	// one channel number may not carry two names.
	Name string
	// Number is the BLE RF channel number.
	Number uint8
	// SpreadFactor is the BLE Coded PHY spreading factor used on this channel. The
	// zero value is config.SpreadFactorS8 (highest range), so a bare
	// Channel{Name: n, Number: m} keeps the historical spreading factor.
	SpreadFactor config.SpreadFactor
}

// Instance is one physical device in the deployment.
type Instance struct {
	// Name uniquely identifies this device within the inventory; it scopes the
	// device's register names in the Registry (e.g. "kitchen").
	Name string
	// UID is the MCU unique ID; the RF address is derived from it.
	UID [config.UIDLen]byte
	// Key is the device's XTEA shared key.
	Key [config.KeyLen]byte
	// Channel is the device's RF channel and the spreading factor it shares with
	// every other node on that channel.
	Channel Channel
	// Type is the device's type (register table).
	Type DeviceType
	// Config is the device-type-specific configuration written to the device's
	// provisioning page. It must be a fixed-size value (see pkg/config); nil means
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
// valid, every instance name is non-empty and unique, and every channel uses a
// single spreading factor (a dongle drives one factor at a time).
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
	if err := inv.validateChannels(); err != nil {
		return err
	}
	if _, err := inv.SpreadFactorByChannel(); err != nil {
		return err
	}
	return nil
}

// validateChannels checks that every channel has a non-empty name and that names
// and numbers form a one-to-one mapping: a channel number must not carry two
// names, and a name must not be reused for two numbers. The hub scopes per-dongle
// diagnostic registers by channel name, so a collision would mix two dongles'
// metrics under one register.
func (inv Inventory) validateChannels() error {
	nameByNum := make(map[uint8]string, len(inv))
	numByName := make(map[string]uint8, len(inv))
	for _, inst := range inv {
		ch := inst.Channel
		if ch.Name == "" {
			return fmt.Errorf("instance %q: channel %d has no name", inst.Name, ch.Number)
		}
		if prev, ok := nameByNum[ch.Number]; ok && prev != ch.Name {
			return fmt.Errorf("channel %d has two names %q and %q", ch.Number, prev, ch.Name)
		}
		if prev, ok := numByName[ch.Name]; ok && prev != ch.Number {
			return fmt.Errorf("channel name %q used for two numbers %d and %d", ch.Name, prev, ch.Number)
		}
		nameByNum[ch.Number] = ch.Name
		numByName[ch.Name] = ch.Number
	}
	return nil
}

// ChannelNames maps each RF channel number in the inventory to its name. The hub
// uses it to label per-dongle diagnostic registers by channel name. Call
// Validate first to ensure the mapping is one-to-one.
func (inv Inventory) ChannelNames() map[uint8]string {
	names := make(map[uint8]string, len(inv))
	for _, inst := range inv {
		names[inst.Channel.Number] = inst.Channel.Name
	}
	return names
}

// SpreadFactorByChannel maps each RF channel number in the inventory to the
// spreading factor its nodes use. It returns an error if two nodes on the same
// channel number disagree: the hub drives one spreading factor per
// channel/dongle, so a channel must be uniform. The hub uses the result to
// configure each dongle.
func (inv Inventory) SpreadFactorByChannel() (map[uint8]config.SpreadFactor, error) {
	byChannel := make(map[uint8]config.SpreadFactor, len(inv))
	owner := make(map[uint8]string, len(inv))
	for _, inst := range inv {
		ch := inst.Channel
		if prev, ok := byChannel[ch.Number]; ok && prev != ch.SpreadFactor {
			return nil, fmt.Errorf("channel %d: instance %q uses spreading factor %d but %q uses %d; "+
				"a channel must use one spreading factor", ch.Number, inst.Name, ch.SpreadFactor, owner[ch.Number], prev)
		}
		byChannel[ch.Number] = ch.SpreadFactor
		owner[ch.Number] = inst.Name
	}
	return byChannel, nil
}
