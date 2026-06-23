package spec

import "github.com/burgrp/bleriot/site/inventory"

type Config struct {
	DefaultRedPeriod   uint32
	DefaultGreenPeriod uint32
}

const (
	RegLedGreen = 1 // green LED period [ms] (0=off, 1=on, >1=blink)
	RegLedRed   = 2 // red LED period [ms] (0=off, 1=on, >1=blink)
	RegGpio     = 3 // GPIO PA0..6 pins state (int)
)

var Chip = inventory.PY32F030

func Type() inventory.DeviceType {
	return inventory.DeviceType{
		Name: "bob",
		Chip: Chip,
		Registers: []inventory.Register{
			{
				Tag:        RegLedGreen,
				Name:       "green",
				Type:       inventory.TypeInt,
				Multiplier: 1,
				Divider:    1,
			},
			{
				Tag:        RegLedRed,
				Name:       "red",
				Type:       inventory.TypeInt,
				Multiplier: 1,
				Divider:    1,
			},
			{
				Tag:        RegGpio,
				Name:       "gpio",
				Type:       inventory.TypeInt,
				Multiplier: 1,
				Divider:    1,
			},
		},
	}
}
