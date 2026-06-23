module github.com/burgrp/bleriot/example/bob

go 1.25.2

require (
	github.com/burgrp/bleriot/lib v0.0.0-00010101000000-000000000000
	github.com/burgrp/tinygo-drivers/bb/spi v0.0.0-20260529225117-75c3fff7a486
	github.com/burgrp/tinygo-drivers/pan211x v0.0.0-20260529225117-75c3fff7a486
)

replace github.com/burgrp/bleriot/lib => ../../lib
