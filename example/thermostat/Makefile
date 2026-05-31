# The thermostat firmware now lives in fw/ (package main). This top-level
# directory is the dual-build library (host type.go + firmware device.go) and has
# no main of its own. Forward the firmware targets to the fw/ module.
.PHONY: build flash gdb rtt install-pack disassembly

build flash gdb rtt install-pack disassembly:
	$(MAKE) -C fw $@
