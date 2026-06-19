// Module site is the BleRiot host library: the host (Linux SBC) half of the
// bridge between BleRiot RF nodes and the external Registry service. A site
// repository imports it and drives it with inventory-as-code, calling
// cli.Start(inventory.Inventory{...}) from its own main().
//
// This module owns all protocol intelligence: per-node XTEA keys, node
// descriptors, retries/timeouts, push-subscription bookkeeping, and the
// Registry client. The radio itself is a USB dongle — an MCP2210 USB-to-SPI
// bridge driving a single PAN211x (no microcontroller, no firmware) — which the
// host drives directly over USB HID (see pkg/mcp2210 and pkg/radio). One dongle
// covers one RF channel; the hub opens one dongle per channel in use.
module site

go 1.25.2

require (
	github.com/burgrp/reg v0.0.0-00010101000000-000000000000
	github.com/burgrp/tinygo-drivers/pan211x v0.0.0-20260529225117-75c3fff7a486
	github.com/lmittmann/tint v1.1.2
	github.com/spf13/cobra v1.10.2
	protocol v0.0.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)

replace protocol => ../protocol

replace github.com/burgrp/reg => ../../reg
