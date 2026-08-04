// Package puya provides chip profiles and on-chip memory-map constants for Puya
// PY32 microcontrollers. It is the BleRiot build/flash/onboarding tooling's
// knowledge of how to build a device's firmware image (TinyGo target), flash it,
// and read its unique ID over SWD (pyocd target, CMSIS pack, UID address).
//
// The profiles are plain inventory.Chip values, so a DeviceType selects one by
// assigning it to DeviceType.Chip and a site may still declare its own chip
// inline. The package is dependency-free and build-tag-free (it only names the
// inventory.Chip type), so it compiles unchanged for both the host and TinyGo.
//
// Memory-map constants are taken from the PY32F030 Datasheet V1.8, §4 "Memory
// Map" (Table 4-1). The system region (0x1FFF_0000..0x1FFF_0FFF) layout is shared
// across the PY32F030 family and reused by the PY32F003 family.
package puya

import "github.com/burgrp/bleriot/lib/shared/inventory"

// On-chip memory map (PY32F030 Datasheet V1.8, §4 "Memory Map", Table 4-1).
// Base addresses are stable across the family; per-part flash and SRAM sizes are
// noted on each chip profile instead.
const (
	// FlashBase is the start of main flash memory (also aliased at 0x0 after boot
	// when configured to boot from flash).
	FlashBase = 0x08000000
	// SRAMBase is the start of SRAM.
	SRAMBase = 0x20000000
	// BootloaderBase is the start of the 3.5 KB factory bootloader (system memory).
	BootloaderBase = 0x1FFF0000
	// UIDAddr is the start of the factory unique ID. The region spans 128 bytes;
	// the first 12 (96-bit, config.UIDLen) are the device's unique ID.
	UIDAddr = 0x1FFF0E00
	// OptionBytesBase is the start of the 128-byte option-byte region.
	OptionBytesBase = 0x1FFF0E80
	// FactoryConfigBase is the start of the 128-byte factory configuration region
	// (HSI trimming and flash-timing parameters).
	FactoryConfigBase = 0x1FFF0F00
)

// PY32F003x6 is the Puya PY32F003x6 chip profile (32 KB flash / 4 KB RAM).
var PY32F003x6 = inventory.Chip{
	Name:         "py32f003x6",
	TinygoTarget: "py32f003x6",
	PyocdTarget:  "py32f003x6",
	CmsisPack:    "PY32F003",
	UIDAddr:      UIDAddr,
}

// PY32F030x8 is the Puya PY32F030x8 chip profile (64 KB flash / 8 KB RAM).
var PY32F030x8 = inventory.Chip{
	Name:         "py32f030x8",
	TinygoTarget: "py32f030_64k_8k",
	PyocdTarget:  "py32f030x8",
	CmsisPack:    "PY32F030",
	UIDAddr:      UIDAddr,
}
