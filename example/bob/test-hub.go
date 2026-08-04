//go:build !tinygo

// Command hub is the example BleRiot site binary. It declares the deployment as
// inventory-as-code and hands it to the shared host runtime, which provides the
// hub, gen and new subcommands.
//
//	go run . new
//	go run . gen
//	go run . hub --registry http://localhost:8080
package main

import (
	"github.com/burgrp/bleriot/lib/shared/config"
	"github.com/burgrp/bleriot/lib/shared/inventory"
	"github.com/burgrp/bleriot/lib/site/cli"

	bob "github.com/burgrp/bleriot/example/bob/spec"
)

// Far and Near are the deployment's RF channels. Each bundles a channel number
// with the spreading factor every node on it uses, so two nodes on one channel
// can never disagree on the factor. Far uses the highest-range S8 factor; Near,
// for nodes close to the hub, uses the faster, shorter-range S2 factor. The
// dongle serving each channel is driven at that channel's factor.
var (
	Far = inventory.Channel{Name: "far", Number: 37, SpreadFactor: config.SpreadFactorS8}
	//Near = inventory.Channel{Name: "near", Number: 38, SpreadFactor: config.SpreadFactorS2}
)

func main() {
	cli.Start(inventory.Inventory{
		{
			Name:    "bob",
			Address: [4]byte{0xCC, 0x81, 0xAF, 0x84},
			Key:     [16]byte{0x04, 0xB8, 0xAF, 0x87, 0x5D, 0x55, 0xFC, 0x76, 0xAC, 0x96, 0x7F, 0xA7, 0x94, 0x20, 0x08, 0x22},
			Channel: Far,
			Type:    bob.Type(),
			Config: bob.Config{
				DefaultRedPeriod:   500,
				DefaultGreenPeriod: 500,
			},
		},
	})
}
