.PHONY: build flash gdb rtt install-pack disassembly new hub

build flash gdb rtt install-pack disassembly:
	@echo "BOB firmware has not migrated to the importable node firmware manifest" >&2
	@false

new:
	go run . node new

hub:
	go run . hub --diagnostics rf
