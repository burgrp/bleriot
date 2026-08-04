# BOB node firmware (flat package main, build tag tinygo). Hardware targets
# (py32, and future stm32/host) are selected by the TinyGo --target and build
# tags rather than separate directories.
#
# The hub's "bleriot make <name> ..." (e.g. "go run . make bob flash") drives this
# Makefile: it writes main_gen.go (the baked-in identity + config) into this
# directory and injects the chip targets (TARGET_TINYGO, TARGET_PYOCD, CMSIS_PACK)
# and the BLERIOT_MAKE sentinel as make variables.
#
# Running make directly (e.g. "make flash") has none of those, so the guard below
# bounces every goal back through "go run . make", which bakes the identity and
# re-enters this Makefile with BLERIOT_MAKE set. Bare "make" builds.
ifndef BLERIOT_MAKE

$(or $(MAKECMDGOALS),build): __bleriot_bounce
	@:
.PHONY: __bleriot_bounce $(MAKECMDGOALS)
__bleriot_bounce:
	@go run . make $(MAKECMDGOALS)

else

IMAGE = image.elf

.PHONY: build flash gdb rtt install-pack disassembly new hub

build:
	tinygo build --stack-size 1024b --size html --serial rtt --scheduler tasks -print-allocs=. --gc leaking --target ${TARGET_TINYGO} -o ${IMAGE} .

flash: build
	pkill pyocd || true
	sleep 1
	pyocd load -t ${TARGET_PYOCD} ${IMAGE}
	pyocd rtt -t ${TARGET_PYOCD}

gdb:
	pyocd gdb --target ${TARGET_PYOCD} --semihosting --persist

rtt:
	pyocd rtt -t ${TARGET_PYOCD}

install-pack:
	pyocd pack install ${CMSIS_PACK}

disassembly: build
	arm-none-eabi-objdump -dS ${IMAGE} > disassembly.txt

new:
	go run . new

hub:
	go run . hub --diagnostics rf

endif
