.PHONY: build flash gdb rtt install-pack disassembly new hub

build:
	go run ./cmd/dev node build --name bob

flash:
	go run ./cmd/dev node build --name bob --flash --rtt

disassembly:
	go run ./cmd/dev node build --name bob --disassembly

gdb rtt install-pack:
	go run ./cmd/dev node $@ --name bob

new:
	go run ./cmd/dev node new

hub:
	go run ./cmd/dev hub --diagnostics rf
