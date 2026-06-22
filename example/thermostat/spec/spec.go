// Package spec is the thermostat's shared device spec: the parts of the device
// definition that both the firmware and the host must agree on. It is imported
// by the firmware (Config + register tags) and the host (everything, including
// Type). The host-only Type and its site/inventory dependency are reachable but
// unused in the firmware, so TinyGo's dead-code elimination strips them.
package spec

import "site/inventory"

// Config is the thermostat's per-device configuration, provisioned into the
// node's flash page and consumed by both the firmware (to clamp setpoints) and
// the host inventory (to declare a device). It is shared so the two sides can
// never disagree on the layout.
type Config struct {
	MinTemp float32
	MaxTemp float32
}

// Register tags are the thermostat's permanent wire identities (protocol/README.md §11):
// unique and non-zero within the device type, and never reused once retired.
//
// This block is the single source of truth for those tags. The host register
// table (Type) and the firmware register handlers (device.go) both reference
// these constants, so the two sides can never drift. They are untyped so they
// fit both inventory.Register.Tag (uint8) and the firmware runtime's uint16.
const (
	TagTemperature = 1 // measured temperature (float, ×0.01 °C)
	TagSetpoint    = 2 // target temperature (float, ×0.01 °C)
	TagHeating     = 3 // heating element on/off (bool)
)

// Chip is the MCU this device's firmware runs on. It is the single source of
// truth shared by both sides: the firmware reads its provisioning page at
// Chip.PageAddr/PageBytes, and the host Type advertises it for provisioning, so
// the two can never disagree on the chip profile.
var Chip = inventory.PY32F030

// Type returns the thermostat device type: its name and register table. Only
// the host needs this register metadata; the firmware just reads/writes
// registers by tag, so TinyGo strips Type from the firmware image.
func Type() inventory.DeviceType {
	return inventory.DeviceType{
		Name: "thermostat",
		Chip: Chip,
		Registers: []inventory.Register{
			{
				Tag:        TagTemperature,
				Name:       "temperature",
				Type:       inventory.TypeFloat,
				Multiplier: 1,
				Divider:    100,
				Metadata:   map[string]string{"unit": "°C"},
			},
			{
				Tag:        TagSetpoint,
				Name:       "setpoint",
				Type:       inventory.TypeFloat,
				Multiplier: 1,
				Divider:    100,
				Metadata:   map[string]string{"unit": "°C"},
			},
			{
				Tag:  TagHeating,
				Name: "heating",
				Type: inventory.TypeBool,
			},
		},
	}
}
