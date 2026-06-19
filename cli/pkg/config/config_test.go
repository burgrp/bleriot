package page

import (
	"bytes"
	"hash/crc32"
	"testing"
)

// sampleConfig is a fixed-size device Config used across the page tests.
type sampleConfig struct {
	Pin      uint8
	_        uint8
	SensorID uint16
	Cal      float32
	Enabled  bool
}

func sampleIdentity() (addr [AddrLen]byte, key [KeyLen]byte, channel uint8) {
	addr = [AddrLen]byte{0xCC, 0xA0, 0x00, 0x02}
	for i := range key {
		key[i] = byte(i)
	}
	channel = 37
	return
}

func TestRoundTrip(t *testing.T) {
	addr, key, channel := sampleIdentity()
	cfg := sampleConfig{Pin: 4, SensorID: 0x1234, Cal: 1.5, Enabled: true}

	data, err := Marshal(addr, key, channel, SpreadFactorS2, cfg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got sampleConfig
	h, err := Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if h.Address != addr || h.Key != key || h.Channel != channel {
		t.Errorf("identity mismatch: %+v", h)
	}
	if h.SpreadFactor != SpreadFactorS2 {
		t.Errorf("spread factor = %d, want %d", h.SpreadFactor, SpreadFactorS2)
	}
	if got != cfg {
		t.Errorf("config = %+v, want %+v", got, cfg)
	}
	if h.Magic != Magic || h.Layout != LayoutVersion {
		t.Errorf("header magic/layout wrong: %+v", h)
	}
}

// TestSpreadFactorDefault checks the zero value round-trips as S8, so an
// inventory that omits the field keeps the historical behaviour.
func TestSpreadFactorDefault(t *testing.T) {
	addr, key, channel := sampleIdentity()
	data, err := Marshal(addr, key, channel, SpreadFactorS8, nil)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	h, err := Unmarshal(data, nil)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if h.SpreadFactor != SpreadFactorS8 || SpreadFactorS8 != 0 {
		t.Errorf("spread factor = %d, want 0 (S8)", h.SpreadFactor)
	}
}

func TestRoundTripNilConfig(t *testing.T) {
	addr, key, channel := sampleIdentity()
	data, err := Marshal(addr, key, channel, SpreadFactorS8, nil)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	h, err := Unmarshal(data, nil)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if h.ConfigLen != 0 {
		t.Errorf("ConfigLen = %d, want 0", h.ConfigLen)
	}
	if h.Address != addr {
		t.Errorf("address mismatch")
	}
}

func TestCRCDetectsCorruption(t *testing.T) {
	addr, key, channel := sampleIdentity()
	data, err := Marshal(addr, key, channel, SpreadFactorS8, sampleConfig{Pin: 9})
	if err != nil {
		t.Fatal(err)
	}
	// Flip a bit inside the config region (after the header).
	data[headerLen] ^= 0x01
	if _, err := Unmarshal(data, &sampleConfig{}); err == nil {
		t.Fatal("expected CRC mismatch error")
	} else if IsUnprovisioned(err) {
		t.Fatal("corruption misclassified as unprovisioned")
	}
}

func TestUnprovisionedZeroPage(t *testing.T) {
	// An erased flash page reads as all-0xFF; a blank one as all-0x00. Neither
	// carries the magic, so both are unprovisioned.
	for _, fill := range []byte{0x00, 0xFF} {
		data := bytes.Repeat([]byte{fill}, headerLen+crcLen+8)
		_, err := Unmarshal(data, &sampleConfig{})
		if err == nil {
			t.Fatalf("fill 0x%02X: expected error", fill)
		}
		if !IsUnprovisioned(err) {
			t.Fatalf("fill 0x%02X: expected unprovisioned, got %v", fill, err)
		}
	}
}

func TestTooShort(t *testing.T) {
	if _, err := Unmarshal([]byte{1, 2, 3}, nil); err == nil {
		t.Fatal("expected error for short page")
	}
}

func TestConfigMustBeFixedSize(t *testing.T) {
	addr, key, channel := sampleIdentity()
	type badConfig struct {
		Name string // variable-size: not allowed
	}
	if _, err := Marshal(addr, key, channel, SpreadFactorS8, badConfig{Name: "x"}); err == nil {
		t.Fatal("expected error for variable-size config")
	}
}

func TestTruncatedConfig(t *testing.T) {
	addr, key, channel := sampleIdentity()
	data, err := Marshal(addr, key, channel, SpreadFactorS8, sampleConfig{Pin: 1})
	if err != nil {
		t.Fatal(err)
	}
	// Drop the trailing CRC and part of the config; ConfigLen now exceeds image.
	truncated := data[:headerLen+1]
	if _, err := Unmarshal(truncated, &sampleConfig{}); err == nil {
		t.Fatal("expected truncation error")
	}
}

// TestDecodeMatchesUnmarshal checks the lean firmware decoder agrees with the
// host Unmarshal on a valid page: same identity, and config bytes that decode to
// the same value.
func TestDecodeMatchesUnmarshal(t *testing.T) {
	addr, key, channel := sampleIdentity()
	cfg := sampleConfig{Pin: 4, SensorID: 0x1234, Cal: 1.5, Enabled: true}
	data, err := Marshal(addr, key, channel, SpreadFactorS2, cfg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	h, cfgBytes, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if h.Address != addr || h.Key != key || h.Channel != channel {
		t.Errorf("identity mismatch: %+v", h)
	}
	if h.SpreadFactor != SpreadFactorS2 {
		t.Errorf("spread factor = %d, want %d", h.SpreadFactor, SpreadFactorS2)
	}
	if h.Magic != Magic || h.Layout != LayoutVersion {
		t.Errorf("header magic/layout wrong: %+v", h)
	}
	// cfgBytes must equal the config region Unmarshal reads.
	if want := data[headerLen : headerLen+int(h.ConfigLen)]; !bytes.Equal(cfgBytes, want) {
		t.Errorf("config bytes = % x, want % x", cfgBytes, want)
	}
}

// TestDecodeToleratesTrailingSlack mirrors how firmware reads a fixed flash
// window larger than the page: extra trailing bytes after the CRC are ignored.
func TestDecodeToleratesTrailingSlack(t *testing.T) {
	addr, key, channel := sampleIdentity()
	data, err := Marshal(addr, key, channel, SpreadFactorS8, sampleConfig{Pin: 7})
	if err != nil {
		t.Fatal(err)
	}
	padded := append(append([]byte{}, data...), make([]byte, 16)...)
	if _, _, err := Decode(padded); err != nil {
		t.Fatalf("Decode with trailing slack: %v", err)
	}
}

func TestDecodeCRCMismatch(t *testing.T) {
	addr, key, channel := sampleIdentity()
	data, err := Marshal(addr, key, channel, SpreadFactorS8, sampleConfig{Pin: 9})
	if err != nil {
		t.Fatal(err)
	}
	data[headerLen] ^= 0x01 // corrupt the config region
	if _, _, err := Decode(data); err == nil {
		t.Fatal("expected CRC mismatch")
	} else if IsUnprovisioned(err) {
		t.Fatal("corruption misclassified as unprovisioned")
	}
}

func TestDecodeUnprovisioned(t *testing.T) {
	for _, fill := range []byte{0x00, 0xFF} {
		data := bytes.Repeat([]byte{fill}, headerLen+crcLen+8)
		if _, _, err := Decode(data); err == nil {
			t.Fatalf("fill 0x%02X: expected error", fill)
		} else if !IsUnprovisioned(err) {
			t.Fatalf("fill 0x%02X: expected unprovisioned, got %v", fill, err)
		}
	}
}

// TestCRC32IEEEMatchesStdlib pins the table-free firmware CRC to the stdlib
// table-based one so the host writer and firmware reader never disagree.
func TestCRC32IEEEMatchesStdlib(t *testing.T) {
	for _, in := range [][]byte{nil, {0}, []byte("BleRiot"), bytes.Repeat([]byte{0xA5}, 64)} {
		if got, want := crc32IEEE(in), crc32.ChecksumIEEE(in); got != want {
			t.Errorf("crc32IEEE(% x) = 0x%08X, want 0x%08X", in, got, want)
		}
	}
}
