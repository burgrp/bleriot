package thermostat

import "site/pkg/inventory"

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
