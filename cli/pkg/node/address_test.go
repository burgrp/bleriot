package node

import (
	"hash/crc32"
	"testing"

	"cli/pkg/config"
)

func TestAddressFromUID(t *testing.T) {
	uid := [config.UIDLen]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}

	got := AddressFromUID(uid)

	// Deterministic and equal to big-endian CRC32 of the UID bytes.
	sum := crc32.ChecksumIEEE(uid[:])
	want := [AddrLen]byte{byte(sum >> 24), byte(sum >> 16), byte(sum >> 8), byte(sum)}
	if got != want {
		t.Fatalf("AddressFromUID = % X, want % X", got, want)
	}

	// Stable across calls.
	if AddressFromUID(uid) != got {
		t.Fatal("AddressFromUID is not deterministic")
	}

	// Different UIDs (very likely) produce different addresses.
	other := uid
	other[0] ^= 0xFF
	if AddressFromUID(other) == got {
		t.Fatal("distinct UIDs produced the same address")
	}
}
