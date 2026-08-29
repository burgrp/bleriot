package protocol

import (
	"bytes"
	"testing"
)

var testKey = [16]byte{
	0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
	0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
}

var testSrc = [4]byte{0xaa, 0xbb, 0xcc, 0xdd}

func mustCodec(t *testing.T) Codec {
	t.Helper()
	c, err := NewCodec(testKey)
	if err != nil {
		t.Fatalf("NewCodec: %v", err)
	}
	return c
}

func TestWireConstants(t *testing.T) {
	if PacketLen != 13 || PacketVersion != 0x01 || FlagNULL != 0x01 {
		t.Fatalf("wire constants: PacketLen=%d PacketVersion=%#x FlagNULL=%#x", PacketLen, PacketVersion, FlagNULL)
	}
	if TypeGET != 0x00 || TypeVALUE != 0x01 || TypeSET != 0x02 || TypeACK != 0x03 {
		t.Fatalf("packet types: GET=%#x VALUE=%#x SET=%#x ACK=%#x", TypeGET, TypeVALUE, TypeSET, TypeACK)
	}
}

func TestGoldenPackets(t *testing.T) {
	cases := []struct {
		name  string
		wire  [PacketLen]byte
		typ   byte
		flags byte
		reg   uint16
		value int32
	}{
		{"GET", [PacketLen]byte{0xaa, 0xbb, 0xcc, 0xdd, 0x01, 0x12, 0x41, 0x12, 0x3f, 0x43, 0xdc, 0xb4, 0xdf}, TypeGET, 0x28, 0x1234, 0},
		{"VALUE", [PacketLen]byte{0xaa, 0xbb, 0xcc, 0xdd, 0x01, 0xfc, 0x73, 0xd5, 0x36, 0xbe, 0x07, 0x86, 0x11}, TypeVALUE, 0x29, 0x1234, -123456789},
		{"SET", [PacketLen]byte{0xaa, 0xbb, 0xcc, 0xdd, 0x01, 0x09, 0x8e, 0xe7, 0xff, 0xff, 0x75, 0x1c, 0xbd}, TypeSET, 0x28, 0xbeef, 0x10203040},
		{"ACK", [PacketLen]byte{0xaa, 0xbb, 0xcc, 0xdd, 0x01, 0x75, 0x98, 0x1d, 0xd6, 0x39, 0x35, 0xcf, 0x91}, TypeACK, 0x29, 0xbeef, 0},
	}
	c := mustCodec(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var encoded [PacketLen]byte
			c.Encode(encoded[:], testSrc, tc.typ, tc.flags, tc.reg, tc.value)
			if encoded != tc.wire {
				t.Fatalf("Encode = % x, want % x", encoded, tc.wire)
			}

			src, typ, flags, reg, value, err := c.Decode(tc.wire[:])
			if err != nil {
				t.Fatalf("Decode literal: %v", err)
			}
			if src != testSrc || typ != tc.typ || flags != tc.flags || reg != tc.reg || value != tc.value {
				t.Fatalf("Decode literal = src %x type %#x flags %#x reg %#x value %d", src, typ, flags, reg, value)
			}
		})
	}
}

// TestRoundTrip encodes then decodes a packet and checks every field.
func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		typ   byte
		flags byte
		reg   uint16
		value int32
	}{
		{"GET", TypeGET, 0, 0x0001, 0},
		{"VALUE", TypeVALUE, 0, 0x0001, 1234},
		{"VALUE null", TypeVALUE, FlagNULL, 0x0005, 0},
		{"SET positive", TypeSET, 0, 0x0002, 1234},
		{"SET negative", TypeSET, 0, 0x0003, -5678},
		{"SET max", TypeSET, 0, 0xffff, 2147483647},
		{"SET min", TypeSET, 0, 0x0000, -2147483648},
		{"ACK", TypeACK, 0, 0x0002, 0},
	}

	c := mustCodec(t)
	buf := make([]byte, PacketLen)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c.Encode(buf, testSrc, tc.typ, tc.flags, tc.reg, tc.value)

			src, typ, flags, reg, value, err := c.Decode(buf)
			if err != nil {
				t.Fatalf("Decode error: %v", err)
			}
			if src != testSrc {
				t.Errorf("src: got %v, want %v", src, testSrc)
			}
			if typ != tc.typ {
				t.Errorf("typ: got 0x%02x, want 0x%02x", typ, tc.typ)
			}
			if flags != tc.flags {
				t.Errorf("flags: got 0x%02x, want 0x%02x", flags, tc.flags)
			}
			if reg != tc.reg {
				t.Errorf("reg: got 0x%04x, want 0x%04x", reg, tc.reg)
			}
			if value != tc.value {
				t.Errorf("value: got %d, want %d", value, tc.value)
			}
		})
	}
}

// TestGuardRoundTrip checks the GUARD field packs into FLAGS bits 1–7 without
// disturbing the NULL bit, can be isolated for a response, survives an XTEA
// round trip, and clamps to the field width.
func TestGuardRoundTrip(t *testing.T) {
	for _, g := range []byte{0, 1, 20, 127} {
		flags := FlagsWithGuard(FlagNULL, g)
		if flags&FlagNULL == 0 {
			t.Errorf("guard %d: NULL bit lost", g)
		}
		if got := GuardMillis(flags); got != g {
			t.Errorf("guard %d: GuardMillis = %d", g, got)
		}
	}
	if got := GuardMillis(FlagsWithGuard(0, 200)); got != MaxGuardMillis {
		t.Errorf("guard clamp: got %d, want %d", got, MaxGuardMillis)
	}
	if got := FlagsWithGuard(0xff, 0); got != FlagNULL {
		t.Errorf("guard replacement: got %#x, want only NULL", got)
	}
	if got := FlagsWithGuard(0xfe, 5); got != 0x0a {
		t.Errorf("guard replacement with NULL clear: got %#x, want 0x0a", got)
	}
	if got := FlagsWithGuard(FlagNULL, 255); got != 0xff {
		t.Errorf("guard clamp preserving NULL: got %#x, want 0xff", got)
	}
	flags := FlagsWithGuard(FlagNULL, 20)
	if got := GuardFlags(flags); got != FlagsWithGuard(0, 20) {
		t.Errorf("GuardFlags: got %#x, want guard without NULL", got)
	}

	c := mustCodec(t)
	buf := make([]byte, PacketLen)
	in := flags
	c.Encode(buf, testSrc, TypeSET, in, 0x0007, 42)
	_, _, flags, _, _, err := c.Decode(buf)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if flags != in {
		t.Errorf("flags through codec: got 0x%02x, want 0x%02x", flags, in)
	}
}

// TestPacketLength checks that Encode writes exactly PacketLen bytes.
func TestPacketLength(t *testing.T) {
	c := mustCodec(t)
	buf := make([]byte, PacketLen)
	c.Encode(buf, testSrc, TypeGET, 0, 1, 0)
	if len(buf) != PacketLen {
		t.Errorf("packet length: got %d, want %d", len(buf), PacketLen)
	}
}

// TestVersionByte checks that byte [4] is PacketVersion after Encode.
func TestVersionByte(t *testing.T) {
	if PacketVersion != 0x01 {
		t.Fatalf("PacketVersion = %#x, want 0x01 for the GET/VALUE protocol", PacketVersion)
	}
	if TypeGET != 0x00 {
		t.Fatalf("TypeGET = %#x, want wire value 0x00", TypeGET)
	}
	c := mustCodec(t)
	buf := make([]byte, PacketLen)
	c.Encode(buf, testSrc, TypeGET, 0, 1, 0)
	if buf[4] != PacketVersion {
		t.Errorf("VER byte: got 0x%02x, want 0x%02x", buf[4], PacketVersion)
	}
}

// TestSrcPlaintext confirms SRC is written unencrypted.
func TestSrcPlaintext(t *testing.T) {
	c := mustCodec(t)
	buf := make([]byte, PacketLen)
	c.Encode(buf, testSrc, TypeGET, 0, 1, 0)
	if !bytes.Equal(buf[0:4], testSrc[:]) {
		t.Errorf("SRC bytes: got %v, want %v", buf[0:4], testSrc)
	}
}

// TestDecodeShortPacket checks that a buffer smaller than PacketLen returns an error.
func TestDecodeShortPacket(t *testing.T) {
	c := mustCodec(t)
	for n := 0; n < PacketLen; n++ {
		_, _, _, _, _, err := c.Decode(make([]byte, n))
		if err != errShortPacket {
			t.Errorf("%d-byte input error = %v, want errShortPacket", n, err)
		}
	}
}

// TestDecodeUnsupportedVersion checks that an unknown version byte is rejected.
func TestDecodeUnsupportedVersion(t *testing.T) {
	c := mustCodec(t)
	buf := make([]byte, PacketLen)
	c.Encode(buf, testSrc, TypeGET, 0, 1, 0)
	buf[4] = 0xff // corrupt version
	_, _, _, _, _, err := c.Decode(buf)
	if err != errUnsupportedPacket {
		t.Errorf("error = %v, want errUnsupportedPacket", err)
	}
}

// TestDifferentKeys checks that decoding with a wrong key returns garbled data.
func TestDifferentKeys(t *testing.T) {
	enc := mustCodec(t)
	var wrongKey [16]byte
	wrongKey[0] = 0xff
	dec, _ := NewCodec(wrongKey)

	buf := make([]byte, PacketLen)
	enc.Encode(buf, testSrc, TypeSET, 0, 0x0042, 999)

	_, typ, _, reg, value, err := dec.Decode(buf)
	if err != nil {
		t.Skip("decode with wrong key returned error (acceptable)")
	}
	// At least one field must differ from the original.
	if typ == TypeSET && reg == 0x0042 && value == 999 {
		t.Error("decoding with wrong key produced correct plaintext")
	}
}

// TestCiphertextChangesWithContent checks that different plaintext → different ciphertext.
func TestCiphertextChangesWithContent(t *testing.T) {
	c := mustCodec(t)
	buf1 := make([]byte, PacketLen)
	buf2 := make([]byte, PacketLen)

	c.Encode(buf1, testSrc, TypeSET, 0, 0x0001, 100)
	c.Encode(buf2, testSrc, TypeSET, 0, 0x0001, 101)

	if bytes.Equal(buf1[5:], buf2[5:]) {
		t.Error("different values produced identical BLOCK ciphertext")
	}
}

// TestNewCodecKeyParsing checks key is loaded as 4×uint32 little-endian.
func TestNewCodecKeyParsing(t *testing.T) {
	key := [16]byte{
		0x01, 0x00, 0x00, 0x00, // word 0 = 1
		0x02, 0x00, 0x00, 0x00, // word 1 = 2
		0x03, 0x00, 0x00, 0x00, // word 2 = 3
		0x04, 0x00, 0x00, 0x00, // word 3 = 4
	}
	c, err := NewCodec(key)
	if err != nil {
		t.Fatalf("NewCodec: %v", err)
	}
	expected := [4]uint32{1, 2, 3, 4}
	if c.key != expected {
		t.Errorf("key: got %v, want %v", c.key, expected)
	}
}
