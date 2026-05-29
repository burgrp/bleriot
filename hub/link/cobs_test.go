package link

import (
	"bytes"
	"testing"
)

func roundTrip(t *testing.T, in []byte) {
	t.Helper()
	enc := cobsEncode(nil, in)
	if bytes.IndexByte(enc, 0) >= 0 {
		t.Fatalf("encoded data contains a zero byte: % x", enc)
	}
	dec, err := cobsDecode(nil, enc)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !bytes.Equal(dec, in) {
		t.Fatalf("round-trip mismatch:\n in:  % x\n out: % x", in, dec)
	}
}

func TestCOBS_RoundTrip(t *testing.T) {
	cases := [][]byte{
		{},
		{0x00},
		{0x01},
		{0x00, 0x00},
		{0x11, 0x22, 0x00, 0x33},
		{0x00, 0x11, 0x00, 0x00, 0x22},
		bytes.Repeat([]byte{0xAA}, 254), // exactly one full 0xFF block
		bytes.Repeat([]byte{0xAA}, 255), // forces a block boundary
		bytes.Repeat([]byte{0xAA}, 600), // multiple full blocks
		bytes.Repeat([]byte{0x00}, 10),  // all zeros
	}
	for _, c := range cases {
		roundTrip(t, c)
	}
}

func TestCOBS_BleRiotPacket(t *testing.T) {
	// A representative 13-byte BleRiot packet with embedded zeros.
	pkt := []byte{0xCC, 0xA0, 0x00, 0x01, 0x00, 0x9F, 0x00, 0x3C, 0x1E, 0x00, 0x8A, 0x2B, 0x00}
	roundTrip(t, pkt)
}

func TestCOBS_AppendsToExisting(t *testing.T) {
	prefix := []byte{0xDE, 0xAD}
	enc := cobsEncode(append([]byte{}, prefix...), []byte{0x01, 0x00, 0x02})
	if !bytes.HasPrefix(enc, prefix) {
		t.Fatalf("encode overwrote existing dst contents: % x", enc)
	}
}

func TestCOBS_DecodeErrors(t *testing.T) {
	if _, err := cobsDecode(nil, []byte{0x00}); err != errZeroCode {
		t.Errorf("zero code: got %v, want errZeroCode", err)
	}
	if _, err := cobsDecode(nil, []byte{0x05, 0x01}); err != errOverrun {
		t.Errorf("overrun: got %v, want errOverrun", err)
	}
}
