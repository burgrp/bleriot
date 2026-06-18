# BleRiot — repository-wide make targets.
#
# Most day-to-day building and testing happens inside the per-module Makefiles
# (cli/, example/...). This top-level Makefile collects targets that span the
# whole repo or need special handling (e.g. hardware-in-the-loop tests).

# USB radio dongle selectors used by the two-dongle functional tests. Each is a
# "scheme:selector" pair: "mcp2210:" plus a USB serial.
DONGLE_HUB  ?= mcp2210:0001746423
DONGLE_NODE ?= mcp2210:0001744916

# functest runs the hardware-in-the-loop functional tests: the real hub engine
# and node runtime exchanging packets over the air across two MCP2210 dongles.
# The tests are gated behind the "dongles" build tag. /dev/hidraw* are root-only,
# so we compile the test binary as the current user (to use the Go toolchain and
# module cache) and run it under sudo.
functest:
	cd cli && go test -c -tags dongles -o /tmp/bleriot-functest.bin ./functest/
	sudo BLERIOT_DONGLE_HUB=$(DONGLE_HUB) BLERIOT_DONGLE_NODE=$(DONGLE_NODE) \
		/tmp/bleriot-functest.bin -test.v -test.timeout 120s

# bench measures end-to-end transaction latency (GET/SET round trips) over the
# real RF link between the two dongles. Same gating and sudo handling as functest.
bench:
	cd cli && go test -c -tags dongles -o /tmp/bleriot-functest.bin ./functest/
	sudo BLERIOT_DONGLE_HUB=$(DONGLE_HUB) BLERIOT_DONGLE_NODE=$(DONGLE_NODE) \
		/tmp/bleriot-functest.bin -test.bench . -test.benchmem -test.run '^$$' -test.benchtime 50x

# test runs the regular (non-hardware) unit tests for the host runtime.
test:
	cd cli && go test ./...

.PHONY: functest bench test

