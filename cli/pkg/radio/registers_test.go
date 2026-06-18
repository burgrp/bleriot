package radio

import (
	"bytes"
	"testing"
)

// fakeTransport records transmitted buffers and returns canned receive data.
type fakeTransport struct {
	sent [][]byte
	// rx maps the access byte to the bytes returned for that transaction.
	rx map[byte][]byte
}

func (f *fakeTransport) Transfer(tx []byte) ([]byte, error) {
	f.sent = append(f.sent, append([]byte(nil), tx...))
	out := make([]byte, len(tx)) // echo length: one rx byte per tx byte
	if canned, ok := f.rx[tx[0]]; ok {
		copy(out, canned)
	}
	return out, nil
}

func TestRegistersRead(t *testing.T) {
	ft := &fakeTransport{rx: map[byte][]byte{
		0x3A << 1: {0x00, 0x5C}, // value clocked in on the trailing byte
	}}
	r := newRegisters(ft)
	v, err := r.Read(0x3A)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if v != 0x5C {
		t.Fatalf("value = 0x%02X, want 0x5C", v)
	}
	if !bytes.Equal(ft.sent[0], []byte{0x3A << 1, 0x00}) {
		t.Fatalf("tx = % X, want access byte then dummy", ft.sent[0])
	}
}

func TestRegistersWrite(t *testing.T) {
	ft := &fakeTransport{}
	r := newRegisters(ft)
	if err := r.Write(0x12, 0xAB); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !bytes.Equal(ft.sent[0], []byte{0x12<<1 | 1, 0xAB}) {
		t.Fatalf("tx = % X, want write access byte then value", ft.sent[0])
	}
}

func TestRegistersWriteBuffer(t *testing.T) {
	ft := &fakeTransport{}
	r := newRegisters(ft)
	if err := r.WriteBuffer(0x20, []byte{1, 2, 3}); err != nil {
		t.Fatalf("WriteBuffer: %v", err)
	}
	if !bytes.Equal(ft.sent[0], []byte{0x20<<1 | 1, 1, 2, 3}) {
		t.Fatalf("tx = % X, want write access byte then data", ft.sent[0])
	}
}

func TestRegistersReadBuffer(t *testing.T) {
	ft := &fakeTransport{rx: map[byte][]byte{
		0x20 << 1: {0x00, 0xDE, 0xAD, 0xBE}, // access byte echo then 3 data bytes
	}}
	r := newRegisters(ft)
	buf := make([]byte, 3)
	if err := r.ReadBuffer(0x20, buf); err != nil {
		t.Fatalf("ReadBuffer: %v", err)
	}
	if !bytes.Equal(buf, []byte{0xDE, 0xAD, 0xBE}) {
		t.Fatalf("buf = % X, want DE AD BE", buf)
	}
	if !bytes.Equal(ft.sent[0], []byte{0x20 << 1, 0, 0, 0}) {
		t.Fatalf("tx = % X, want read access byte then dummies", ft.sent[0])
	}
}
