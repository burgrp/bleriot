// Package page defines the BleRiot provisioning page: a fixed flash region
// written to a device once at provisioning time and read once at boot by the
// firmware. It carries the device identity (RF address, XTEA key, channel) and
// the device-type-specific Config.
//
// The same Marshal/Unmarshal code compiles on the host (which packs the page and
// writes it over SWD) and in TinyGo firmware (which reads it at boot), so the two
// can never disagree about the byte layout.
//
// The page layout is:
//
//	header  (fixed)  magic | layout | configLen | channel | pad | address | key
//	config  (varies) the device type's fixed-size Config struct, binary-encoded
//	crc32   (u32)    CRC-32 (IEEE) over everything before it
//
// Config must be a fixed-size value: only fixed-width integers, floats, bools,
// and arrays/structs of those. Slices, strings, maps and pointers are rejected,
// because the firmware decodes the page without a heap allocator.
package page

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
)

// Wire constants and field widths.
const (
	// Magic marks a written page ("BLRT"). An all-zero / erased flash page has a
	// different magic and is treated as unprovisioned.
	Magic uint32 = 0x424C5254

	// LayoutVersion guards the header layout. Firmware refuses a page whose layout
	// it does not understand.
	LayoutVersion uint16 = 1

	// UIDLen is the MCU unique-ID length in bytes (used to derive the RF address).
	UIDLen = 12
	// AddrLen is the BleRiot RF address length in bytes.
	AddrLen = 4
	// KeyLen is the XTEA shared-key length in bytes.
	KeyLen = 16

	// headerLen is the encoded size of Header: magic(4) layout(2) configLen(2)
	// channel(1) pad(1) address(4) key(16) = 30.
	headerLen = 4 + 2 + 2 + 1 + 1 + AddrLen + KeyLen
	// crcLen is the trailing CRC-32 size.
	crcLen = 4
)

// byteOrder is the page byte order. Little-endian matches the Cortex-M target so
// the firmware can, if it wants, overlay fields without byte swaps.
var byteOrder = binary.LittleEndian

// Header is the fixed-size identity prefix of a provisioning page.
type Header struct {
	Magic     uint32
	Layout    uint16
	ConfigLen uint16
	Channel   uint8
	_         uint8 // padding to keep Address 16-bit aligned
	Address   [AddrLen]byte
	Key       [KeyLen]byte
}

// errUnprovisioned reports that a page was never written (magic mismatch).
var errUnprovisioned = errors.New("page: unprovisioned (magic mismatch)")

// IsUnprovisioned reports whether err indicates an unwritten/erased page, as
// opposed to a corrupt or incompatible one. Firmware uses this to distinguish
// "fresh device, wait for provisioning" from "bad page".
func IsUnprovisioned(err error) bool { return errors.Is(err, errUnprovisioned) }

// Marshal encodes the identity and config into a page image:
// header || config || crc32. config may be nil (no config bytes); otherwise it
// must be a fixed-size value (see the package doc).
func Marshal(addr [AddrLen]byte, key [KeyLen]byte, channel uint8, config any) ([]byte, error) {
	var cfg bytes.Buffer
	if config != nil {
		if err := binary.Write(&cfg, byteOrder, config); err != nil {
			return nil, fmt.Errorf("page: encode config (must be a fixed-size struct): %w", err)
		}
	}
	if cfg.Len() > int(^uint16(0)) {
		return nil, fmt.Errorf("page: config too large (%d bytes)", cfg.Len())
	}

	h := Header{
		Magic:     Magic,
		Layout:    LayoutVersion,
		ConfigLen: uint16(cfg.Len()),
		Channel:   channel,
		Address:   addr,
		Key:       key,
	}

	var out bytes.Buffer
	if err := binary.Write(&out, byteOrder, &h); err != nil {
		return nil, fmt.Errorf("page: encode header: %w", err)
	}
	out.Write(cfg.Bytes())
	if err := binary.Write(&out, byteOrder, crc32.ChecksumIEEE(out.Bytes())); err != nil {
		return nil, fmt.Errorf("page: encode crc: %w", err)
	}
	return out.Bytes(), nil
}

// Unmarshal validates a page image (magic, layout, length, CRC) and decodes the
// config into config, which must be a pointer to a fixed-size value matching the
// one passed to Marshal (or nil to skip config decoding). It returns the
// identity Header.
//
// If the page was never written, the returned error satisfies IsUnprovisioned.
func Unmarshal(data []byte, config any) (Header, error) {
	var h Header
	if len(data) < headerLen+crcLen {
		return h, errors.New("page: too short")
	}
	if err := binary.Read(bytes.NewReader(data[:headerLen]), byteOrder, &h); err != nil {
		return h, fmt.Errorf("page: decode header: %w", err)
	}
	if h.Magic != Magic {
		return h, errUnprovisioned
	}
	if h.Layout != LayoutVersion {
		return h, fmt.Errorf("page: layout v%d, firmware expects v%d", h.Layout, LayoutVersion)
	}

	end := headerLen + int(h.ConfigLen)
	if len(data) < end+crcLen {
		return h, errors.New("page: truncated (config length exceeds image)")
	}
	got := byteOrder.Uint32(data[end : end+crcLen])
	if want := crc32.ChecksumIEEE(data[:end]); got != want {
		return h, fmt.Errorf("page: CRC mismatch (got 0x%08X, want 0x%08X)", got, want)
	}

	if config != nil && h.ConfigLen > 0 {
		if err := binary.Read(bytes.NewReader(data[headerLen:end]), byteOrder, config); err != nil {
			return h, fmt.Errorf("page: decode config: %w", err)
		}
	}
	return h, nil
}
