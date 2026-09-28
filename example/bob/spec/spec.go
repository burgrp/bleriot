package spec

import (
	"github.com/burgrp/bleriot/lib/shared/firmware"
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
		Firmware: firmware.Manifest{
			Package: "github.com/burgrp/bleriot/example/bob",
			TinyGo: firmware.TinyGoProfile{
				Scheduler:        firmware.SchedulerTasks,
				StackSizeBytes:   1024,
				GarbageCollector: firmware.GCLeaking,
				Serial:           firmware.SerialRTT,
				SizeReport:       firmware.SizeReportHTML,
				PrintAllocs:      true,
			},
			PyOCD: firmware.PyOCDProfile{
				RTTMode:                  firmware.ConnectUnderReset,
				Reclaim:                  true,
				ReclaimDelayMilliseconds: 1000,
			},
		},
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
