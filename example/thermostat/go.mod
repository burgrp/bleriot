module thermostat

go 1.25.2

require cli v0.0.0

// cli and its host dependencies are only needed by the host-only type.go
// (build tag !tinygo); the firmware build excludes them.
replace cli => ../../cli

replace protocol => ../../protocol

replace hub/link => ../../hub/link

replace github.com/burgrp/reg => ../../../reg
