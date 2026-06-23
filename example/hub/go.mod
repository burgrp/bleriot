module github.com/burgrp/bleriot/example/hub

go 1.25.2

require (
	github.com/burgrp/bleriot/site v0.0.0
)

require (
	github.com/burgrp/reg v0.0.0-00010101000000-000000000000 // indirect
	github.com/burgrp/tinygo-drivers/pan211x v0.0.0-20260529225117-75c3fff7a486 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lmittmann/tint v1.1.2 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/burgrp/bleriot/protocol v0.0.0 // indirect
)

replace github.com/burgrp/bleriot/site => ../../site

replace github.com/burgrp/bleriot/protocol => ../../protocol

replace github.com/burgrp/reg => ../../../reg

