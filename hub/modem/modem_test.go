package modem

import (
	"context"
	"net"
	"testing"
	"time"

	"link"
)

// mcuSide simulates the firmware end of the link: it decodes frames the host
// sends and lets the test inject frames back to the host.
type mcuSide struct {
	conn net.Conn
	dec  *link.Decoder
}

func newPair(t *testing.T) (*Modem, *mcuSide, context.CancelFunc) {
	t.Helper()
	hostConn, mcuConn := net.Pipe()
	m := New(hostConn, 8)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = m.Run(ctx) }()
	return m, &mcuSide{conn: mcuConn, dec: link.NewDecoder(maxFrame)}, cancel
}

// readMsg reads bytes from the host until one complete message is decoded.
func (s *mcuSide) readMsg(t *testing.T) link.Message {
	t.Helper()
	s.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	b := make([]byte, 1)
	for {
		n, err := s.conn.Read(b)
		if err != nil {
			t.Fatalf("mcu read: %v", err)
		}
		if n == 0 {
			continue
		}
		msg, ok, derr := s.dec.Push(b[0])
		if derr != nil {
			t.Fatalf("mcu decode: %v", derr)
		}
		if ok {
			return msg
		}
	}
}

// send injects a frame from the MCU to the host.
func (s *mcuSide) send(t *testing.T, msg link.Message) {
	t.Helper()
	frame := link.AppendMessage(nil, msg)
	s.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := s.conn.Write(frame); err != nil {
		t.Fatalf("mcu write: %v", err)
	}
}

func TestModem_ConfigRadio(t *testing.T) {
	m, mcu, cancel := newPair(t)
	defer cancel()

	addr := [4]byte{0xCC, 0xA0, 0x00, 0x02}
	go func() { _ = m.ConfigRadio(10, addr) }()

	got := mcu.readMsg(t)
	if got.Type != link.MsgConfigRadio || got.Channel != 10 || got.Addr != addr {
		t.Fatalf("unexpected ConfigRadio frame: %+v", got)
	}
}

func TestModem_Send(t *testing.T) {
	m, mcu, cancel := newPair(t)
	defer cancel()

	dst := [4]byte{0xDE, 0xAD, 0xBE, 0xEF}
	pkt := []byte{0xCC, 0xA0, 0x00, 0x01, 0x00, 0x9F, 0x3C, 0x1E, 0x8A, 0x2B, 0x7D, 0x4F, 0x06}
	go func() { _ = m.Send(dst, pkt) }()

	got := mcu.readMsg(t)
	if got.Type != link.MsgSend || got.Addr != dst {
		t.Fatalf("unexpected Send frame: %+v", got)
	}
	if len(got.Payload) != PacketLen {
		t.Fatalf("payload length = %d, want %d", len(got.Payload), PacketLen)
	}
}

func TestModem_SendRejectsWrongLength(t *testing.T) {
	m, _, cancel := newPair(t)
	defer cancel()
	if err := m.Send([4]byte{}, []byte{1, 2, 3}); err == nil {
		t.Fatal("expected error for short payload")
	}
}

func TestModem_ReceivesPackets(t *testing.T) {
	m, mcu, cancel := newPair(t)
	defer cancel()

	pkt := [PacketLen]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13}
	mcu.send(t, link.Message{Type: link.MsgRecv, Payload: pkt[:]})

	select {
	case got := <-m.Received():
		if got != pkt {
			t.Fatalf("received %v, want %v", got, pkt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for received packet")
	}
}

func TestModem_Hello(t *testing.T) {
	m, mcu, cancel := newPair(t)
	defer cancel()

	mcu.send(t, link.Message{Type: link.MsgHello, Version: link.ProtocolVersion})

	deadline := time.After(2 * time.Second)
	for m.ProtocolVersion() == 0 {
		select {
		case <-deadline:
			t.Fatal("protocol version not updated after hello")
		case <-time.After(5 * time.Millisecond):
		}
	}
	if m.ProtocolVersion() != link.ProtocolVersion {
		t.Fatalf("version = %d, want %d", m.ProtocolVersion(), link.ProtocolVersion)
	}
}

func TestModem_Errors(t *testing.T) {
	m, mcu, cancel := newPair(t)
	defer cancel()

	mcu.send(t, link.Message{Type: link.MsgError, Code: link.ErrTxFailed})

	select {
	case code := <-m.Errors():
		if code != link.ErrTxFailed {
			t.Fatalf("error code = %d, want %d", code, link.ErrTxFailed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for error")
	}
}

func TestModem_CloseOnCancel(t *testing.T) {
	m, _, cancel := newPair(t)
	cancel()

	// Received channel must close once Run returns.
	select {
	case _, open := <-m.Received():
		if open {
			// drain any buffered packet, then ensure it closes
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Received channel not closed after cancel")
	}

	// Subsequent sends must fail.
	deadline := time.After(2 * time.Second)
	for {
		err := m.Send([4]byte{}, make([]byte, PacketLen))
		if err == ErrClosed {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("Send did not return ErrClosed, got %v", err)
		case <-time.After(5 * time.Millisecond):
		}
	}
}
