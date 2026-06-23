// Module shared is the neutral, dependency-free implementation of the BleRiot
// RF wire format (shared/README.md §4–§8): packet encode/decode and XTEA.
//
// It is shared by both the node firmware and the host hub
// (site), so the on-wire format is single-sourced. It has no external
// dependencies and no build tags, so it compiles for the host and under TinyGo
// alike.
module github.com/burgrp/bleriot/lib

go 1.25.2

require (
	github.com/burgrp/reg v1.0.12
	github.com/burgrp/tinygo-drivers/pan211x v0.0.0-20260529225117-75c3fff7a486
	github.com/lmittmann/tint v1.1.3
	github.com/spf13/cobra v1.10.2
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)
