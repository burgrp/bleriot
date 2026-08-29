# BleRiot — repository-wide make targets.
#
# Most day-to-day building and testing happens inside the per-module Makefiles
# (site/, example/...). This top-level Makefile collects targets that span the
# whole repo or need special handling (e.g. hardware-in-the-loop tests).

# USB radio dongle selectors used by the two-dongle functional tests. Each is a
# "scheme:selector" pair: "mcp2210:" plus a USB serial. These tests bypass the
# hub command's dynamic discovery: one dongle directly runs the hub radio and
# the other directly emulates a node, so their test roles are intentionally
# fixed.
DONGLE_HUB  ?= mcp2210:0001746423
DONGLE_NODE ?= mcp2210:0001744916

# functest runs the hardware-in-the-loop functional tests: the real hub engine
# and node runtime exchanging packets over the air across two MCP2210 dongles.
# The tests are gated behind the "dongles" build tag. The dongles' /dev/hidraw*
# nodes are owned by the plugdev group (see usb/99-bleriot-mcp2210.rules), so the
# tests run directly as the current user — no sudo needed.
functest:
	cd lib/site && BLERIOT_DONGLE_HUB=$(DONGLE_HUB) BLERIOT_DONGLE_NODE=$(DONGLE_NODE) \
		go test -tags dongles -v -timeout 120s ./functest/

# bench measures end-to-end transaction latency (GET/SET round trips) over the
# real RF link between the two dongles. Same gating as functest, also no sudo.
bench:
	cd lib/site && BLERIOT_DONGLE_HUB=$(DONGLE_HUB) BLERIOT_DONGLE_NODE=$(DONGLE_NODE) \
		go test -tags dongles -bench . -benchmem -run '^$$' -benchtime 50x ./functest/

# test runs every regular host-compatible unit test. The PAN211x node adapter is
# TinyGo-only, so it is intentionally excluded from the standard Go package set.
test:
	cd lib && go test ./node ./shared/... ./site/...

.PHONY: functest bench test

