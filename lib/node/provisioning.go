package node

import "github.com/burgrp/bleriot/lib/shared/config"

// Provisioning is a node's per-device identity, baked into the firmware image by
// the host "gen" command (see lib/site/cli) rather than read from flash at boot.
// The generated main() constructs one of these and hands it to the firmware's
// bleriotMain entry point together with the device config.
//
// Address is derived from the MCU unique ID (AddressFromUID) by the host; the
// firmware can re-derive it from its own UID to verify the image was flashed to
// the intended chip.
type Provisioning struct {
	// Address is the node's 4-byte RF address (§3), AddressFromUID(UID).
	Address [config.AddrLen]byte
	// Key is the node's 16-byte XTEA shared key (§5); secret.
	Key [config.KeyLen]byte
	// Channel is the BLE RF channel the node listens and transmits on.
	Channel uint8
	// SpreadFactor is the BLE Coded PHY spreading factor for the channel.
	SpreadFactor config.SpreadFactor
}

// AddressFromUID derives a node's 4-byte RF address from its 12-byte MCU unique
// ID (lib/README.md §11.5): address = CRC32(UID), big-endian. It matches the
// host-side lib/site/node.AddressFromUID (hash/crc32.ChecksumIEEE) bit for bit,
// so the firmware's boot-time self-check agrees with the address the host baked
// in. The CRC is computed bitwise (no 1 KiB lookup table) to keep the firmware
// small; it runs once at boot, so the per-bit loop's cost is irrelevant.
func AddressFromUID(uid [config.UIDLen]byte) [config.AddrLen]byte {
	crc := crc32IEEE(uid[:])
	return [config.AddrLen]byte{
		byte(crc >> 24),
		byte(crc >> 16),
		byte(crc >> 8),
		byte(crc),
	}
}

// crc32IEEE computes the IEEE CRC-32 bitwise (the same value as
// hash/crc32.ChecksumIEEE) without a lookup table.
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
