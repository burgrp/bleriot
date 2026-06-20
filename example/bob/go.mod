module bob

go 1.25.2

require (
	github.com/burgrp/tinygo-drivers/bb/spi v0.0.0-20260529225117-75c3fff7a486
	github.com/burgrp/tinygo-drivers/pan211x v0.0.0-20260529225117-75c3fff7a486
	protocol v0.0.0
	site v0.0.0
)

// The firmware (package main, build tag tinygo) and the host-facing spec
// subpackage (thermostat/spec, build tag !tinygo for the site/inventory parts)
// share one module. site and protocol resolve to the local checkouts; the
// firmware build, driven by the go.work workspace, also uses the local
// tinygo-drivers checkouts.
replace site => ../../site

replace protocol => ../../protocol

replace github.com/burgrp/reg => ../../../reg
