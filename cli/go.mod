// Module cli is the BleRiot host library: the host (Linux SBC) half of the
// bridge between BleRiot RF nodes and the external Registry service. A site
// repository imports it and drives it with inventory-as-code, calling
// host.Start(inventory.Inventory{...}) from its own main().
//
// The hub is split into two cooperating parts (see the protocol spec):
//
//   - This host module owns all protocol intelligence: per-node XTEA keys,
//     node descriptors, retries/timeouts, push-subscription bookkeeping, and
//     the Registry client.
//   - A separate MCU "dumb radio modem" (TinyGo firmware) owns only the
//     PAN211x radios and holds no secrets.
//
// The two halves communicate over a COBS-framed byte stream (UART now, USB-CDC
// later) defined in the standalone hub/link module. Each modem manages exactly
// one radio over its own serial port; the host fans out across several modems
// (one per port), so that multiplexing lives in the host above the link layer,
// not on the wire.
module cli

go 1.25.2

require (
	github.com/burgrp/reg v0.0.0-00010101000000-000000000000
	github.com/lmittmann/tint v1.1.2
	github.com/spf13/cobra v1.10.2
	go.bug.st/serial v1.6.4
	hub/link v0.0.0
	protocol v0.0.0
)

require (
	github.com/creack/goselect v0.1.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/sys v0.38.0 // indirect
)

replace protocol => ../protocol

replace hub/link => ../hub/link

replace github.com/burgrp/reg => ../../reg
