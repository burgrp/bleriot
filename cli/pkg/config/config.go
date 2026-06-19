// Package config defines the BleRiot provisioning page: a fixed flash region
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
//	header  (fixed)  magic | layout | configLen | channel | spreadFactor | address | key
//	config  (varies) the device type's fixed-size Config struct, binary-encoded
//	crc32   (u32)    CRC-32 (IEEE) over everything before it
//
// Config must be a fixed-size value: only fixed-width integers, floats, bools,
// and arrays/structs of those. Slices, strings, maps and pointers are rejected,
// because the firmware decodes the page without a heap allocator.
package config

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
	// it does not understand. Bumped to 2 when the spare header pad byte became the
	// SpreadFactor field.
	LayoutVersion uint16 = 2

	// UIDLen is the MCU unique-ID length in bytes (used to derive the RF address).
	UIDLen = 12
	// AddrLen is the BleRiot RF address length in bytes.
	AddrLen = 4
	// KeyLen is the XTEA shared-key length in bytes.
	KeyLen = 16

	// headerLen is the encoded size of Header: magic(4) layout(2) configLen(2)
	// channel(1) spreadFactor(1) address(4) key(16) = 30.
	headerLen = 4 + 2 + 2 + 1 + 1 + AddrLen + KeyLen
	// crcLen is the trailing CRC-32 size.
	crcLen = 4
)

// byteOrder is the page byte order. Little-endian matches the Cortex-M target so
// the firmware can, if it wants, overlay fields without byte swaps.
var byteOrder = binary.LittleEndian

// SpreadFactor selects the PAN211x BLE Coded PHY spreading factor: the trade-off
// between range and on-air time. A higher spreading factor reaches farther but
// keeps the radio transmitting longer; a lower one is faster but shorter range.
//
// The values match github.com/burgrp/tinygo-drivers/pan211x.SpreadFactor so both
// the host dongle and the firmware can convert with a plain cast. The zero value
// is SpreadFactorS8, so an inventory that omits the field keeps the historical
// behaviour.
type SpreadFactor uint8

const (
	// SpreadFactorS8 is S=8: highest range, ~125 kbps, longest on-air time. Default.
	SpreadFactorS8 SpreadFactor = 0
	// SpreadFactorS2 is S=2: medium range, ~500 kbps, shorter on-air time.
	SpreadFactorS2 SpreadFactor = 1
)

// Header is the fixed-size identity prefix of a provisioning page.
type Header struct {
	Magic        uint32
	Layout       uint16
	ConfigLen    uint16
	Channel      uint8
	SpreadFactor SpreadFactor // occupies the byte that keeps Address 16-bit aligned
	Address      [AddrLen]byte
	Key          [KeyLen]byte
}

// errUnprovisioned reports that a page was never written (magic mismatch).
var errUnprovisioned = errors.New("config: unprovisioned (magic mismatch)")

// IsUnprovisioned reports whether err indicates an unwritten/erased page, as
// opposed to a corrupt or incompatible one. Firmware uses this to distinguish
// "fresh device, wait for provisioning" from "bad page".
func IsUnprovisioned(err error) bool { return errors.Is(err, errUnprovisioned) }

// Marshal encodes the identity and config into a page image:
// header || config || crc32. config may be nil (no config bytes); otherwise it
// must be a fixed-size value (see the package doc).
func Marshal(addr [AddrLen]byte, key [KeyLen]byte, channel uint8, spreadFactor SpreadFactor, cfgVal any) ([]byte, error) {
	var cfg bytes.Buffer
	if cfgVal != nil {
		if err := binary.Write(&cfg, byteOrder, cfgVal); err != nil {
			return nil, fmt.Errorf("config: encode config (must be a fixed-size struct): %w", err)
		}
	}
	if cfg.Len() > int(^uint16(0)) {
		return nil, fmt.Errorf("config: config too large (%d bytes)", cfg.Len())
	}

	h := Header{
		Magic:        Magic,
		Layout:       LayoutVersion,
		ConfigLen:    uint16(cfg.Len()),
		Channel:      channel,
		SpreadFactor: spreadFactor,
		Address:      addr,
		Key:          key,
	}

	var out bytes.Buffer
	if err := binary.Write(&out, byteOrder, &h); err != nil {
		return nil, fmt.Errorf("config: encode header: %w", err)
	}
	out.Write(cfg.Bytes())
	if err := binary.Write(&out, byteOrder, crc32.ChecksumIEEE(out.Bytes())); err != nil {
		return nil, fmt.Errorf("config: encode crc: %w", err)
	}
	return out.Bytes(), nil
}

// Unmarshal validates a page image (magic, layout, length, CRC) and decodes the
// config into cfg, which must be a pointer to a fixed-size value matching the
// one passed to Marshal (or nil to skip config decoding). It returns the
// identity Header.
//
// If the page was never written, the returned error satisfies IsUnprovisioned.
func Unmarshal(data []byte, cfg any) (Header, error) {
	var h Header
	if len(data) < headerLen+crcLen {
		return h, errors.New("config: too short")
	}
	if err := binary.Read(bytes.NewReader(data[:headerLen]), byteOrder, &h); err != nil {
		return h, fmt.Errorf("config: decode header: %w", err)
	}
	if h.Magic != Magic {
		return h, errUnprovisioned
	}
	if h.Layout != LayoutVersion {
		return h, fmt.Errorf("config: layout v%d, firmware expects v%d", h.Layout, LayoutVersion)
	}

	end := headerLen + int(h.ConfigLen)
	if len(data) < end+crcLen {
		return h, errors.New("config: truncated (config length exceeds image)")
	}
	got := byteOrder.Uint32(data[end : end+crcLen])
	if want := crc32.ChecksumIEEE(data[:end]); got != want {
		return h, fmt.Errorf("config: CRC mismatch (got 0x%08X, want 0x%08X)", got, want)
	}

	if cfg != nil && h.ConfigLen > 0 {
		if err := binary.Read(bytes.NewReader(data[headerLen:end]), byteOrder, cfg); err != nil {
			return h, fmt.Errorf("config: decode config: %w", err)
		}
	}
	return h, nil
}

// Sentinel errors returned by Decode. They carry no formatting so the firmware
// decode path pulls in neither fmt nor reflection.
var (
	errTooShort = errors.New("config: too short")
	errLayout   = errors.New("config: unsupported layout")
	errCRC      = errors.New("config: CRC mismatch")
)

// Decode validates a page image and returns its identity header and the raw
// config bytes (aliasing data), without using reflection, fmt, allocation, or a
// CRC lookup table. It is the firmware-side counterpart to Unmarshal: it lets
// TinyGo dead-code-eliminate the host-only Marshal/Unmarshal path, keeping the
// firmware small. The caller decodes the config bytes into its own fixed-size
// Config (the layout matches Marshal: fields in declaration order, little-endian).
//
// If the page was never written, the returned error satisfies IsUnprovisioned.
func Decode(data []byte) (Header, []byte, error) {
	var h Header
	if len(data) < headerLen+crcLen {
		return h, nil, errTooShort
	}
	h.Magic = byteOrder.Uint32(data[0:4])
	if h.Magic != Magic {
		return h, nil, errUnprovisioned
	}
	h.Layout = byteOrder.Uint16(data[4:6])
	if h.Layout != LayoutVersion {
		return h, nil, errLayout
	}
	h.ConfigLen = byteOrder.Uint16(data[6:8])
	h.Channel = data[8]
	h.SpreadFactor = SpreadFactor(data[9])
	copy(h.Address[:], data[10:10+AddrLen])
	copy(h.Key[:], data[10+AddrLen:10+AddrLen+KeyLen])

	end := headerLen + int(h.ConfigLen)
	if len(data) < end+crcLen {
		return h, nil, errTooShort
	}
	if byteOrder.Uint32(data[end:end+crcLen]) != crc32IEEE(data[:end]) {
		return h, nil, errCRC
	}
	return h, data[headerLen:end], nil
}

// crc32IEEE computes the IEEE CRC-32 bitwise (the same value as
// hash/crc32.ChecksumIEEE) without a 1 KiB lookup table, for the firmware decode
// path. Decoding one small page at boot makes the per-bit loop's cost
// irrelevant.
func crc32IEEE(data []byte) uint32 {
	crc := ^uint32(0)
	for _, b := range data {
		crc ^= uint32(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xEDB88320
			} else {
				crc >>= 1
			}
		}
	}
	return ^crc
}
