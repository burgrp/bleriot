// Package puya provides chip profiles and on-chip memory-map constants for Puya
// PY32 microcontrollers. It is BleRiot's knowledge of how to build a device's
// firmware image (TinyGo target) and flash it (pyocd target and CMSIS pack).
//
// The profiles are plain inventory.Chip values, so a DeviceType selects one by
// assigning it to DeviceType.Chip and a site may still declare its own chip
// inline. The package is dependency-free and build-tag-free (it only names the
// inventory.Chip type), so it compiles unchanged for both the host and TinyGo.
//
// Each MCU family lives in its own file (py32f002.go, py32f003.go,
// py32f030.go) with its chip profiles and the memory-map constants
// taken from that family's datasheet. The flash and SRAM base addresses below are
// common to every PY32F0 family; the system region (UID, option and factory
// bytes) is declared per family because its layout differs — notably PY32F002B,
// which places the UID at the base of the system region rather than 0x1FFF0E00.
package puya

// Flash and SRAM base addresses, common to every PY32F0 family. Per-family flash
// and SRAM sizes are noted on each chip profile.
const (
	FlashBase = 0x08000000 // start of main flash (aliased at 0x0 after boot)
	SRAMBase  = 0x20000000 // start of SRAM
)
