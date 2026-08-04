package node

import (
	"hash/crc32"
	"testing"

	"github.com/burgrp/bleriot/lib/shared/config"
)

// TestAddressFromUIDMatchesStdlib pins the firmware's table-free address
// derivation to the host's (hash/crc32.ChecksumIEEE, big-endian), so the boot
// self-check agrees with the address the host bakes into the image.
func TestAddressFromUIDMatchesStdlib(t *testing.T) {
	uids := [][config.UIDLen]byte{
		{},
		{0x5A, 0x33, 0x50, 0x41, 0x12, 0x32, 0x35, 0x32, 0x29, 0x93, 0x95, 0x00},
		{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
	}
	for _, uid := range uids {
		crc := crc32.ChecksumIEEE(uid[:])
		want := [config.AddrLen]byte{byte(crc >> 24), byte(crc >> 16), byte(crc >> 8), byte(crc)}
		if got := AddressFromUID(uid); got != want {
			t.Errorf("AddressFromUID(%X) = %X, want %X", uid, got, want)
		}
	}
}

// TestCRC32IEEEMatchesStdlib checks the bitwise CRC used by the firmware equals
// the stdlib table-based one for assorted inputs.
func TestCRC32IEEEMatchesStdlib(t *testing.T) {
	for _, in := range [][]byte{nil, {0}, []byte("BleRiot"), {0xA5, 0xA5, 0xA5, 0xA5}} {
		if got, want := crc32IEEE(in), crc32.ChecksumIEEE(in); got != want {
			t.Errorf("crc32IEEE(% x) = 0x%08X, want 0x%08X", in, got, want)
		}
	}
}
