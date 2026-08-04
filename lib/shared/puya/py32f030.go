package puya

import "github.com/burgrp/bleriot/lib/shared/inventory"

// PY32F030 system region (PY32F030 Datasheet V1.8, §4 "Memory Map", Table 4-1).
const (
	F030UIDAddr           = 0x1FFF0E00 // 128-byte region; first 12 bytes are the UID
	F030OptionBytesBase   = 0x1FFF0E80 // 128-byte option bytes
	F030FactoryConfigBase = 0x1FFF0F00 // 128-byte factory config (HSI trim, flash timing)
	F030BootloaderBase    = 0x1FFF0000 // 3.5 KB factory bootloader (system memory)
)

// The PY32F030 family shares one die; members differ only in fused-off flash and
// SRAM (Datasheet §1 product tables, cross-checked against the Puya.PY32F0xx CMSIS
// pack). The density suffix maps: x4=16K/2K, x6=32K/4K, x7=48K/6K, x8=64K/8K. All
// use the "PY32F030" CMSIS pack.

// PY32F030x4 is the Puya PY32F030x4 chip profile (16 KB flash / 2 KB RAM).
var PY32F030x4 = inventory.Chip{
	Name:         "py32f030x4",
	TinygoTarget: "py32f030x4",
	PyocdTarget:  "py32f030x4",
	CmsisPack:    "PY32F030",
}

// PY32F030x6 is the Puya PY32F030x6 chip profile (32 KB flash / 4 KB RAM).
var PY32F030x6 = inventory.Chip{
	Name:         "py32f030x6",
	TinygoTarget: "py32f030x6",
	PyocdTarget:  "py32f030x6",
	CmsisPack:    "PY32F030",
}

// PY32F030x7 is the Puya PY32F030x7 chip profile (48 KB flash / 6 KB RAM).
var PY32F030x7 = inventory.Chip{
	Name:         "py32f030x7",
	TinygoTarget: "py32f030x7",
	PyocdTarget:  "py32f030x7",
	CmsisPack:    "PY32F030",
}

// PY32F030x8 is the Puya PY32F030x8 chip profile (64 KB flash / 8 KB RAM).
var PY32F030x8 = inventory.Chip{
	Name:         "py32f030x8",
	TinygoTarget: "py32f030x8",
	PyocdTarget:  "py32f030x8",
	CmsisPack:    "PY32F030",
}
