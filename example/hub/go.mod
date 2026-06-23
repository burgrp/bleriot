module github.com/burgrp/bleriot/example/hub

go 1.25.2

require (
	github.com/burgrp/bleriot/example/bob v0.0.0-00010101000000-000000000000
	github.com/burgrp/bleriot/lib v0.0.0-00010101000000-000000000000
)

require (
	github.com/burgrp/reg v1.0.12 // indirect
	github.com/burgrp/tinygo-drivers/pan211x v0.0.0-20260529225117-75c3fff7a486 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lmittmann/tint v1.1.3 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)

replace github.com/burgrp/bleriot/example/bob => ../bob

replace github.com/burgrp/bleriot/lib => ../../lib
