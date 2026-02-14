# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an embedded firmware project targeting the **PY32F030 microcontroller** (ARM Cortex-M0+, 64KB flash, 8KB RAM) built with **TinyGo**. The firmware runs bare-metal with no operating system.

## Hardware Target

- **MCU**: Puya PY32F030 (specifically py32f030_64k_8k variant)
- **Board**: embedFire PY32F030 development board
- **Documentation**: Available at `/home/paul/lib/MCU/Puya/`
  - [PY32F030_Datasheet_V1.8.pdf](/home/paul/lib/MCU/Puya/PY32F030_Datasheet_V1.8.pdf)
  - [PY32F030 Reference manual v1.3_EN.pdf](/home/paul/lib/MCU/Puya/PY32F030%20Reference%20manual%20v1.3_EN.pdf)

## Build System

The project uses a Makefile-based build system with TinyGo and PyOCD:

### Common Commands

```bash
# Build firmware (generates image.elf)
make build

# Build and flash to MCU
make flash

# Start GDB debug session with semihosting
make gdb

# Monitor RTT (Real-Time Transfer) output
make rtt

# Generate disassembly listing
make disassembly

# Install CMSIS-Pack for the MCU (one-time setup)
make install-pack
```

### Build Configuration

Key variables in [Makefile](Makefile):
- `TARGET_TINYGO`: py32f030_64k_8k (TinyGo target specification)
- `TARGET_PYOCD`: py32f030x8 (PyOCD target for flashing/debugging)
- `CMSIS_PACK`: PY32F030 (CMSIS-Pack identifier)

TinyGo build flags:
- `--scheduler tasks`: Task-based cooperative scheduler
- `--gc leaking`: No garbage collection (bare-metal)
- `--serial uart`: UART for serial output
- `--size html`: Generates size-report.html showing memory usage

## Architecture

### Pin Configuration

Hardware pins are defined as constants in main.go:
- **UART**: PB6 (TX), PB7 (RX) - for serial communication
- **LEDs**: PB0 (Red), PB1 (Green) - status indicators

### TinyGo Machine Package

The project uses TinyGo's `machine` package for hardware abstraction:
- `machine.Pin` - GPIO control
- `machine.ConfigureUARTPin()` - UART setup
- Pin modes: `machine.PinOutput`, `machine.PinInput`

### VSCode Integration

- **Default build task**: `make flash` (Ctrl+Shift+B)
- **Go tooling**: Configured to use TinyGo's GOROOT with ARM/Cortex-M tags
- See [.vscode/settings.json](.vscode/settings.json) for complete build tags

## Development Workflow

1. **Write firmware code** in main.go or new .go files
2. **Build**: `make build` - generates image.elf and size-report.html
3. **Flash**: `make flash` - uploads to connected MCU via PyOCD
4. **Debug**: `make gdb` - start GDB session, or `make rtt` for RTT output

### Build Artifacts

- `image.elf`: Compiled firmware binary
- `size-report.html`: Interactive memory usage visualization
- `disassembly.txt`: ARM assembly listing (generated on-demand)

## Parent Project Context

This `test-fw` directory is part of a larger project at `/home/paul/git/reg24/`:
- **scanner/**: BLE scanner utility (TinyGo + bluetooth package)
- **bob/**: KiCad hardware design files for the PCB
- **sub/hw-kicad**: Git submodule with hardware component libraries

## Important Notes

- The build uses **bare-metal** configuration (no OS, no standard runtime)
- Memory is extremely limited (8KB RAM) - avoid heap allocations
- The scheduler is **cooperative** - long-running operations must yield
- Serial output requires UART pins to be configured before use
