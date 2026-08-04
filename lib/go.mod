// Module lib is the BleRiot library: the single Go module holding the shared RF
// wire format, the firmware-side runtime, and the host hub.
//
//   - lib/shared — neutral, dependency-free, build-tag-free packages shared by
//     firmware and host (protocol codec + XTEA, identity primitives, the
//     inventory-as-code model). The on-wire formats are single-sourced here and
//     compile for the host and under TinyGo alike (see lib/README.md, the
//     protocol specification).
//   - lib/node — the firmware-side BleRiot runtime (receive/dispatch loop, XTEA
//     codec, GET/SET/WATCH), imported by node firmware.
//   - lib/site — the host (Linux-SBC) hub library: engine, radio dongle drivers,
//     firmware identity generation and the Registry bridge (see lib/site/README.md).
module github.com/burgrp/bleriot/lib

go 1.25.2

require (
	github.com/burgrp/reg v1.0.12
	github.com/burgrp/tinygo-drivers/bb/spi v1.0.0
	github.com/burgrp/tinygo-drivers/pan211x v1.0.0
	github.com/lmittmann/tint v1.1.3
	github.com/spf13/cobra v1.10.2
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)

// Design phase: consume the in-tree pan211x driver (SetChannelBLE/SetChannelRF)
// directly from its local checkout instead of a tagged release.
// replace github.com/burgrp/tinygo-drivers/pan211x => /home/paul/git/tinygo-drivers/pan211x

// replace github.com/burgrp/tinygo-drivers/bb/spi => /home/paul/git/tinygo-drivers/bb/spi
