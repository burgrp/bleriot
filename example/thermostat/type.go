package thermostat

import "cli/pkg/inventory"

// Type returns the thermostat device type: its name and register table. It is
// host-only (excluded from the firmware build) because only the host needs the
// register metadata; the firmware just reads/writes registers by tag.
//
// Register tags are permanent wire identities (PROTOCOL.md): unique, non-zero,
// and never reused once retired.
func Type() inventory.DeviceType {
	return inventory.DeviceType{
		Name: "thermostat",
		Chip: inventory.PY32F030,
		Registers: []inventory.Register{
			{
				Tag:        1,
				Name:       "temperature",
				Type:       inventory.TypeFloat,
				Multiplier: 1,
				Divider:    100,
				Metadata:   map[string]string{"unit": "°C"},
			},
			{
				Tag:        2,
				Name:       "setpoint",
				Type:       inventory.TypeFloat,
				Multiplier: 1,
				Divider:    100,
				Metadata:   map[string]string{"unit": "°C"},
			},
			{
				Tag:  3,
				Name: "heating",
				Type: inventory.TypeBool,
			},
		},
	}
}
