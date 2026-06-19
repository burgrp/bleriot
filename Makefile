# BleRiot — repository-wide make targets.
#
# Most day-to-day building and testing happens inside the per-module Makefiles
# (site/, example/...). This top-level Makefile collects targets that span the
# whole repo or need special handling (e.g. hardware-in-the-loop tests).

# USB radio dongle selectors used by the two-dongle functional tests. Each is a
# "scheme:selector" pair: "mcp2210:" plus a USB serial.
DONGLE_HUB  ?= mcp2210:0001746423
DONGLE_NODE ?= mcp2210:0001744916

# functest runs the hardware-in-the-loop functional tests: the real hub engine
# and node runtime exchanging packets over the air across two MCP2210 dongles.
# The tests are gated behind the "dongles" build tag. The dongles' /dev/hidraw*
# nodes are owned by the plugdev group (see usb/99-bleriot-mcp2210.rules), so the
# tests run directly as the current user — no sudo needed.
functest:
	cd site && BLERIOT_DONGLE_HUB=$(DONGLE_HUB) BLERIOT_DONGLE_NODE=$(DONGLE_NODE) \
		go test -tags dongles -v -timeout 120s ./functest/

# bench measures end-to-end transaction latency (GET/SET round trips) over the
# real RF link between the two dongles. Same gating as functest, also no sudo.
bench:
	cd site && BLERIOT_DONGLE_HUB=$(DONGLE_HUB) BLERIOT_DONGLE_NODE=$(DONGLE_NODE) \
		go test -tags dongles -bench . -benchmem -run '^$$' -benchtime 50x ./functest/

# test runs the regular (non-hardware) unit tests for the host runtime.
test:
	cd site && go test ./...

.PHONY: functest bench test

