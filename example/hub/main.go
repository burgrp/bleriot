// Command hub is the example BleRiot site binary. It declares the deployment as
// inventory-as-code and hands it to the shared host runtime, which provides the
// hub, provision and new subcommands.
//
//	go run . new
//	go run . provision
//	go run . hub --registry http://localhost:8080 --dongle mcp2210:/dev/hidraw0,37
package main

import (
	"site/cli"
	"site/config"
	"site/inventory"

	"thermostat"
)

// Far and Near are the deployment's RF channels. Each bundles a channel number
// with the spreading factor every node on it uses, so two nodes on one channel
// can never disagree on the factor. Far uses the highest-range S8 factor; Near,
// for nodes close to the hub, uses the faster, shorter-range S2 factor. The
// dongle serving each channel is driven at that channel's factor.
var (
	Far  = inventory.Channel{Number: 37, SpreadFactor: config.SpreadFactorS8}
	Near = inventory.Channel{Number: 38, SpreadFactor: config.SpreadFactorS2}
)

func main() {
	cli.Start(inventory.Inventory{
		{
			Name:    "kitchen",
			UID:     [12]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B},
			Key:     [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F},
			Channel: Far,
			Type:    thermostat.Type(),
			Config: thermostat.Config{
				MinTemp: 18.0,
				MaxTemp: 22.0,
			},
		},
		{
			Name:    "living_room",
			UID:     [12]byte{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B},
			Key:     [16]byte{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F},
			Channel: Far,
			Type:    thermostat.Type(),
			Config: thermostat.Config{
				MinTemp: 20.0,
				MaxTemp: 23.0,
			},
		},
		{
			Name:    "lab",
			UID:     [12]byte{0x5A, 0x33, 0x50, 0x41, 0x12, 0x32, 0x35, 0x32, 0x29, 0x93, 0x95, 0x00},
			Key:     [16]byte{0x04, 0xB8, 0xAF, 0x87, 0x5D, 0x55, 0xFC, 0x76, 0xAC, 0x96, 0x7F, 0xA7, 0x94, 0x20, 0x08, 0x22},
			Channel: Far,
			Type:    thermostat.Type(),
			Config: thermostat.Config{
				MinTemp: 19.0,
				MaxTemp: 21.0,
			},
		},
		{
			// A second bench board, close to the hub, so it sits on the Near
			// channel and uses its faster short-range S2 spreading factor.
			Name:    "bench",
			UID:     [12]byte{0x5A, 0x33, 0x50, 0x41, 0x12, 0x32, 0x35, 0x32, 0x29, 0x93, 0x4E, 0x00},
			Key:     [16]byte{0x72, 0x28, 0x7D, 0xBA, 0x69, 0x31, 0x5A, 0x3E, 0xA0, 0xC3, 0x26, 0x77, 0x43, 0xB0, 0x3E, 0xAC},
			Channel: Near,
			Type:    thermostat.Type(),
			Config: thermostat.Config{
				MinTemp: 19.0,
				MaxTemp: 21.0,
			},
		},
	})
}
