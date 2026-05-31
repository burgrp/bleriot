module hub

go 1.25.2

require (
	cli v0.0.0
	thermostat v0.0.0
)

require (
	github.com/burgrp/reg v0.0.0-00010101000000-000000000000 // indirect
	github.com/creack/goselect v0.1.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lmittmann/tint v1.1.2 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	go.bug.st/serial v1.6.4 // indirect
	golang.org/x/sys v0.38.0 // indirect
	hub/link v0.0.0 // indirect
	protocol v0.0.0 // indirect
)

replace cli => ../../cli

replace thermostat => ../thermostat

replace protocol => ../../protocol

replace hub/link => ../../hub/link

replace github.com/burgrp/reg => ../../../reg
