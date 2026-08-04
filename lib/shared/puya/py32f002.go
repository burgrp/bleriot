package puya

import "github.com/burgrp/bleriot/lib/shared/inventory"

// PY32F002A system region (PY32F002A Datasheet Rev.0.2, §4 "Memory Map",
// Table 4-1). The layout matches PY32F030/PY32F003; only the bootloader is
// smaller.
const (
	F002AUIDAddr           = 0x1FFF0E00 // 128-byte region; first 12 bytes are the UID
	F002AOptionBytesBase   = 0x1FFF0E80 // 128-byte option bytes
	F002AFactoryConfigBase = 0x1FFF0F00 // 128-byte factory config (HSI trim)
	F002ABootloaderBase    = 0x1FFF0000 // 2 KB factory bootloader (system memory)
)

// PY32F002Ax5 is the Puya PY32F002Ax5 chip profile (20 KB flash / 3 KB RAM).
var PY32F002Ax5 = inventory.Chip{
	Name:         "py32f002ax5",
	TinygoTarget: "py32f002ax5",
	PyocdTarget:  "py32f002ax5",
	CmsisPack:    "PY32F002A",
	UIDAddr:      F002AUIDAddr,
}

// PY32F002B system region (PY32F002B Datasheet V1.8, §4 "Memory Map", Table 4-1).
// Unlike the other families the UID sits at the base of the system region
// (0x1FFF0000), not at 0x1FFF0E00, so its constants differ.
const (
	F002BUIDAddr            = 0x1FFF0000 // 128-byte region; first 12 bytes are the UID
	F002BOptionBytesBase    = 0x1FFF0080 // 128-byte option bytes
	F002BFactoryConfig0Base = 0x1FFF0100 // HSI trim and flash erase/write timing
	F002BFactoryConfig1Base = 0x1FFF0180 // trim data and power-on verification code
	F002BUserOTPBase        = 0x1FFF0280 // 128-byte user OTP memory
)

// PY32F002Bx5 is the Puya PY32F002Bx5 chip profile (24 KB flash / 3 KB RAM).
var PY32F002Bx5 = inventory.Chip{
	Name:         "py32f002bx5",
	TinygoTarget: "py32f002bx5",
	PyocdTarget:  "py32f002bx5",
	CmsisPack:    "PY32F002B",
	UIDAddr:      F002BUIDAddr,
}
