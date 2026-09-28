// Command dev is the local BOB inventory and BleRiot CLI.
package main

import (
	"github.com/burgrp/bleriot/example/bob/spec"
	"github.com/burgrp/bleriot/lib/shared/config"
	"github.com/burgrp/bleriot/lib/shared/inventory"
	"github.com/burgrp/bleriot/lib/site/cli"
)

var (
	Far  = inventory.Channel{Name: "far", Number: 37, SpreadFactor: config.SpreadFactorS8}
	Near = inventory.Channel{Name: "near", Number: 38, SpreadFactor: config.SpreadFactorS2}
)

func main() {
	cli.Start(inventory.Inventory{
		{
			Name:    "bob",
			Address: [4]byte{0xCC, 0x81, 0xAF, 0x84},
			Key:     [16]byte{0x04, 0xB8, 0xAF, 0x87, 0x5D, 0x55, 0xFC, 0x76, 0xAC, 0x96, 0x7F, 0xA7, 0x94, 0x20, 0x08, 0x22},
			Channel: Far,
			Type:    spec.Type(),
			Config: spec.Config{
				DefaultLedPeriod: 500,
			},
		},
		// {
		// 	Name:    "bench",
		// 	Address: [4]byte{0xAE, 0x4D, 0xB3, 0x50},
		// 	Key:     [16]byte{0x72, 0x28, 0x7D, 0xBA, 0x69, 0x31, 0x5A, 0x3E, 0xA0, 0xC3, 0x26, 0x77, 0x43, 0xB0, 0x3E, 0xAC},
		// 	Channel: Near,
		// 	Type:    spec.Type(),
		// 	Config: spec.Config{
		// 		DefaultLedPeriod: 100,
		// 	},
		// },
	})
}
