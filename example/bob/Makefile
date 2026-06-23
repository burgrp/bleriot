# BOB node firmware (flat package main, build tag tinygo). Hardware targets
# (py32, and future stm32/host) are selected by the TinyGo --target and build
# tags rather than separate directories.
TARGET_TINYGO = py32f030_64k_8k
TARGET_PYOCD = py32f030x8
CMSIS_PACK = PY32F030

IMAGE = image.elf

.PHONY: build flash gdb rtt install-pack disassembly

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
