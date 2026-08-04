# BOB node firmware (flat package main, build tag tinygo). Hardware targets
# (py32, and future stm32/host) are selected by the TinyGo --target and build
# tags rather than separate directories.
#
# The target names and the provisioning generator are overridable so the hub's
# "bleriot make <name> ..." can inject a device's chip targets and bake its
# identity from the hub inventory; standalone "make flash" uses the defaults
# below and this directory's own inventory.
TARGET_TINYGO ?= py32f030_64k_8k
TARGET_PYOCD ?= py32f030x8
CMSIS_PACK ?= PY32F030
GEN ?= go run . gen

IMAGE = image.elf

.PHONY: build flash gdb rtt install-pack disassembly new gen hub

# gen bakes the inventory instance's identity + config into a generated file the
# firmware build compiles in. Write via a temp file so a stale, valid
# provisioning_gen.go is never truncated to an empty (unparseable) file while
# "go run" is compiling.
gen:
	${GEN} > provisioning_gen.go.tmp && mv provisioning_gen.go.tmp provisioning_gen.go

build: gen
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
