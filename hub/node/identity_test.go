package node

import "testing"

func TestParseAddress(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    [AddrLen]byte
		wantErr bool
	}{
		{name: "plain hex", in: "A3F2B841", want: [AddrLen]byte{0xA3, 0xF2, 0xB8, 0x41}},
		{name: "0x prefix", in: "0xA3F2B841", want: [AddrLen]byte{0xA3, 0xF2, 0xB8, 0x41}},
		{name: "0X prefix", in: "0XCCA00002", want: [AddrLen]byte{0xCC, 0xA0, 0x00, 0x02}},
		{name: "lowercase", in: "ccaa0001", want: [AddrLen]byte{0xCC, 0xAA, 0x00, 0x01}},
		{name: "too short", in: "A3F2B8", wantErr: true},
		{name: "too long", in: "A3F2B84100", wantErr: true},
		{name: "odd length", in: "A3F2B8412", wantErr: true},
		{name: "non-hex", in: "ZZZZZZZZ", wantErr: true},
		{name: "empty", in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAddress(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseAddress(%q) = %v, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAddress(%q): unexpected error %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("ParseAddress(%q) = % X, want % X", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseIdentity(t *testing.T) {
	const goodKey = "00112233445566778899AABBCCDDEEFF"

	t.Run("valid", func(t *testing.T) {
		id, err := ParseIdentity("0xCCA00002", goodKey)
		if err != nil {
			t.Fatalf("ParseIdentity: %v", err)
		}
		if id.Address != ([AddrLen]byte{0xCC, 0xA0, 0x00, 0x02}) {
			t.Errorf("address = % X", id.Address)
		}
		want := [KeyLen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
			0x88, 0x99, 0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
		if id.Key != want {
			t.Errorf("key = % X, want % X", id.Key, want)
		}
	})

	t.Run("bad address", func(t *testing.T) {
		if _, err := ParseIdentity("ZZ", goodKey); err == nil {
			t.Fatal("expected error for bad address")
		}
	})

	t.Run("short key", func(t *testing.T) {
		if _, err := ParseIdentity("CCA00002", "0011223344"); err == nil {
			t.Fatal("expected error for short key")
		}
	})

	t.Run("non-hex key", func(t *testing.T) {
		if _, err := ParseIdentity("CCA00002", "ZZ112233445566778899AABBCCDDEEFF"); err == nil {
			t.Fatal("expected error for non-hex key")
		}
	})
}

func TestNewNode(t *testing.T) {
	d := &Descriptor{Channel: 37}
	id := Identity{Address: [AddrLen]byte{1, 2, 3, 4}}
	n := NewNode("thermo", d, id)
	if n.Descriptor != d {
		t.Error("descriptor not embedded")
	}
	if n.Address != id.Address {
		t.Error("identity not embedded")
	}
	if n.Name != "thermo" {
		t.Errorf("node name = %q, want \"thermo\"", n.Name)
	}
	// Fields of the embedded descriptor are reachable directly.
	if n.Channel != 37 {
		t.Errorf("embedded descriptor field: channel=%d", n.Channel)
	}
}
