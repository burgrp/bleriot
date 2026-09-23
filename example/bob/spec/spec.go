package spec

import (
	"github.com/burgrp/bleriot/lib/shared/inventory"
	"github.com/burgrp/bleriot/lib/shared/puya"
)

type Config struct {
	DefaultLedPeriod uint32
}

// Tag 2 is retired; permanent register tags are never reassigned.
const (
	RegLed  = 1 // green LED period [ms] (0=off, 1=on, >1=blink)
	RegGpio = 3 // GPIO PA0..6 pins state (int)
)

var Chip = puya.PY32F030x8

func Type() inventory.DeviceType {
	return inventory.DeviceType{
		Name: "bob",
		Chip: Chip,
		Registers: []inventory.Register{
			{
				Tag:  RegLed,
				Name: "led",
				Type: inventory.TypeInt,
			},
			{
				Tag:      RegGpio,
				Name:     "gpio",
				Type:     inventory.TypeInt,
				ReadOnly: true,
			},
		},
	}
}
