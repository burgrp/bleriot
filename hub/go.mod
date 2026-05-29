// Module hub is the BleRiot hub: the host (Linux SBC) half of the bridge
// between BleRiot RF nodes and the external Registry service.
//
// The hub is split into two cooperating parts (see PROTOCOL.md):
//
//   - This host module owns all protocol intelligence: per-node XTEA keys,
//     node descriptors, retries/timeouts, push-subscription bookkeeping, and
//     the Registry client.
//   - A separate MCU "dumb radio modem" (TinyGo firmware) owns only the
//     PAN211x radios and holds no secrets.
//
// The two halves communicate over a COBS-framed byte stream (UART now, USB-CDC
// later) defined in package hub/link. Each modem manages exactly one radio over
// its own serial port; the host fans out across several modems (one per port),
// so that multiplexing lives in the host above the link layer, not on the wire.
module hub

go 1.25.2

require (
	bleriot v0.0.0
	github.com/burgrp/reg v0.0.0-00010101000000-000000000000
	go.bug.st/serial v1.6.4
)

require (
	github.com/creack/goselect v0.1.2 // indirect
	golang.org/x/sys v0.38.0 // indirect
)

replace bleriot => ../bleriot

replace github.com/burgrp/reg => ../../reg
