package link

import (
	"bytes"
	"testing"
)

// feed pushes every byte of stream into d, returning all successfully decoded
// messages and the first error encountered (if any).
func feed(d *Decoder, stream []byte) ([]Message, error) {
	var out []Message
	var firstErr error
	for _, b := range stream {
		msg, ok, err := d.Push(b)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if ok {
			// Copy payload since it aliases internal storage.
			if msg.Payload != nil {
				msg.Payload = append([]byte(nil), msg.Payload...)
			}
			out = append(out, msg)
		}
	}
	return out, firstErr
}

func TestFrame_RoundTripAllTypes(t *testing.T) {
	msgs := []Message{
		{Type: MsgHello, Version: ProtocolVersion},
		{Type: MsgConfigRadio, Channel: 10, Addr: [4]byte{0xCC, 0xA0, 0x00, 0x02}},
		{Type: MsgSend, Addr: [4]byte{0xDE, 0xAD, 0xBE, 0xEF},
			Payload: []byte{0x00, 0x11, 0x00, 0x22, 0x33, 0x00, 0x44, 0x55, 0x66, 0x77, 0x88, 0x00, 0x99}},
		{Type: MsgRecv,
			Payload: []byte{0xCC, 0xA0, 0x00, 0x01, 0x00, 0x9F, 0x3C, 0x1E, 0x8A, 0x2B, 0x7D, 0x4F, 0x06}},
		{Type: MsgError, Code: ErrTxFailed},
	}

	var stream []byte
	for _, m := range msgs {
		stream = AppendMessage(stream, m)
	}

	d := NewDecoder(64)
	got, err := feed(d, stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(msgs) {
		t.Fatalf("got %d messages, want %d", len(got), len(msgs))
	}
	for i, want := range msgs {
		g := got[i]
		if g.Type != want.Type || g.Version != want.Version || g.Channel != want.Channel ||
			g.Code != want.Code || g.Addr != want.Addr {
			t.Errorf("message %d header mismatch:\n got  %+v\n want %+v", i, g, want)
		}
		if !bytes.Equal(g.Payload, want.Payload) {
			t.Errorf("message %d payload mismatch:\n got  % x\n want % x", i, g.Payload, want.Payload)
		}
	}
}

func TestFrame_NoZeroBytesInEncoding(t *testing.T) {
	stream := AppendMessage(nil, Message{Type: MsgRecv,
		Payload: []byte{0x00, 0x00, 0x00, 0x01}})
	// Only the trailing delimiter may be zero.
	body := stream[:len(stream)-1]
	if bytes.IndexByte(body, 0) >= 0 {
		t.Errorf("encoded frame body contains zero byte: % x", stream)
	}
	if stream[len(stream)-1] != Delimiter {
		t.Errorf("frame not delimiter-terminated: % x", stream)
	}
}

func TestFrame_ResyncAfterGarbage(t *testing.T) {
	good := AppendMessage(nil, Message{Type: MsgHello, Version: 2})

	d := NewDecoder(64)
	// Leading garbage bytes (no delimiter) then a delimiter to flush, then a
	// valid frame. The garbage frame should error but not break the next one.
	stream := append([]byte{0x05, 0x06, 0x00}, good...)
	got, _ := feed(d, stream)
	if len(got) != 1 || got[0].Type != MsgHello || got[0].Version != 2 {
		t.Fatalf("decoder failed to resync after garbage: %+v", got)
	}
}

func TestFrame_EmptyFramesIgnored(t *testing.T) {
	d := NewDecoder(64)
	got, err := feed(d, []byte{0x00, 0x00, 0x00})
	if err != nil {
		t.Fatalf("empty frames should not error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("empty frames should yield no messages, got %d", len(got))
	}
}

func TestFrame_OverlongFrameDropped(t *testing.T) {
	d := NewDecoder(8)
	// Build a frame whose payload exceeds maxFrame, then a valid one after.
	big := AppendMessage(nil, Message{Type: MsgSend,
		Addr: [4]byte{1, 2, 3, 4}, Payload: bytes.Repeat([]byte{0xAB}, 32)})
	good := AppendMessage(nil, Message{Type: MsgHello, Version: 1})

	got, _ := feed(d, append(big, good...))
	// The oversized frame must be dropped; the following valid frame must decode.
	if len(got) != 1 || got[0].Type != MsgHello {
		t.Fatalf("expected only the valid frame to survive, got %+v", got)
	}
}

func TestParseMessage_Errors(t *testing.T) {
	if _, err := parseMessage(nil); err != errShortBody {
		t.Errorf("empty body: got %v, want errShortBody", err)
	}
	if _, err := parseMessage([]byte{byte(MsgConfigRadio), 0x01}); err != errShortBody {
		t.Errorf("short ConfigRadio: got %v, want errShortBody", err)
	}
	if _, err := parseMessage([]byte{0x7F}); err != errUnknownType {
		t.Errorf("unknown type: got %v, want errUnknownType", err)
	}
}
