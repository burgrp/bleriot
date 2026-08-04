// Package config defines shared, build-tag-free primitives used by both the
// BleRiot firmware and host: the RF/identity field widths and the RF spreading
// factor.
//
// A device's identity (RF address, XTEA key, channel, spread factor) and its
// device-type Config are no longer serialized to a flash page; they are baked
// into the per-device firmware image by the host "gen" command (see
// lib/site/cli) as ordinary Go values. This package therefore carries only the
// small primitives both sides must agree on.
package config

// Field widths shared by firmware and host.
const (
	// UIDLen is the MCU unique-ID length in bytes (used to derive the RF address).
	UIDLen = 12
	// AddrLen is the BleRiot RF address length in bytes.
	AddrLen = 4
	// KeyLen is the XTEA shared-key length in bytes.
	KeyLen = 16
)

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

// String returns the symbolic spread-factor name ("S8"/"S2") so logs read
// clearly instead of showing the raw 0/1. Every return is a compile-time string
// literal that lives in read-only flash, so firmware use adds no RAM. It also
// makes SpreadFactor a fmt.Stringer, which slog/fmt call automatically.
func (s SpreadFactor) String() string {
	switch s {
	case SpreadFactorS8:
		return "S8"
	case SpreadFactorS2:
		return "S2"
	default:
		return "S?"
	}
}
