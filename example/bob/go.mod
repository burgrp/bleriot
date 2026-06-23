module github.com/burgrp/bleriot/example/bob

go 1.25.2

require (
	github.com/burgrp/bleriot/shared v0.0.0
	github.com/burgrp/tinygo-drivers/bb/spi v0.0.0-20260529225117-75c3fff7a486
	github.com/burgrp/tinygo-drivers/pan211x v0.0.0-20260529225117-75c3fff7a486
)

replace github.com/burgrp/bleriot/site => ../../site

replace github.com/burgrp/bleriot/shared => ../../shared
