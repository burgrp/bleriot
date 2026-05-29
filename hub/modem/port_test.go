package modem

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"link"
)

// writeFrame encodes msg and writes it to the mcu side of a pipe.
func writeFrame(t *testing.T, c net.Conn, msg link.Message) {
	t.Helper()
	buf := link.AppendMessage(nil, msg)
	_ = c.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.Write(buf); err != nil {
		t.Fatalf("mcu write: %v", err)
	}
}

// readFrame reads bytes from the mcu side until one complete message decodes.
func readFrame(t *testing.T, c net.Conn, d *link.Decoder) link.Message {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	b := make([]byte, 1)
	for {
		n, err := c.Read(b)
		if err != nil {
			t.Fatalf("mcu read: %v", err)
		}
		if n == 0 {
			continue
		}
		msg, ok, derr := d.Push(b[0])
		if derr != nil {
			continue
		}
		if ok {
			return msg
		}
	}
}

func TestPort_StartsDisconnectedAndReconnects(t *testing.T) {
	mcuCh := make(chan net.Conn, 4)
	var allow atomic.Bool // false until we make the "device" available

	open := func() (io.ReadWriteCloser, error) {
		if !allow.Load() {
			return nil, errors.New("device not present")
		}
		host, mcu := net.Pipe()
		mcuCh <- mcu
		return host, nil
	}

	p := NewPort(PortConfig{
		Open:       open,
		Channel:    37,
		Addr:       [link.AddrLen]byte{9, 9, 9, 9},
		RecvBuffer: 4,
		Backoff:    5 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go p.Run(ctx)

	// While no device is present the Port keeps running and Send fails cleanly.
	if err := p.Send([link.AddrLen]byte{1, 2, 3, 4}, make([]byte, PacketLen)); !errors.Is(err, ErrDisconnected) {
		t.Fatalf("Send while disconnected = %v, want ErrDisconnected", err)
	}

	// Make the device available; the Port should connect on a later attempt.
	allow.Store(true)
	mcu := waitConn(t, mcuCh)

	// On connect the Port configures the radio.
	d := link.NewDecoder(maxFrame)
	if msg := readFrame(t, mcu, d); msg.Type != link.MsgConfigRadio || msg.Channel != 37 {
		t.Fatalf("expected ConfigRadio ch37 on connect, got %+v", msg)
	}

	// A packet from the radio is delivered on Received().
	var pkt [PacketLen]byte
	pkt[0] = 0xAB
	writeFrame(t, mcu, link.Message{Type: link.MsgRecv, Payload: pkt[:]})
	if got := waitPkt(t, p.Received()); got != pkt {
		t.Fatalf("received %v, want %v", got, pkt)
	}

	// Simulate the transport dropping: closing the mcu side fails the host read.
	_ = mcu.Close()

	// The Port reconnects and reconfigures the new radio.
	mcu2 := waitConn(t, mcuCh)
	d2 := link.NewDecoder(maxFrame)
	if msg := readFrame(t, mcu2, d2); msg.Type != link.MsgConfigRadio {
		t.Fatalf("expected reconfigure after reconnect, got %+v", msg)
	}

	// The same stable Received() channel keeps delivering after the reconnect.
	var pkt2 [PacketLen]byte
	pkt2[0] = 0xCD
	writeFrame(t, mcu2, link.Message{Type: link.MsgRecv, Payload: pkt2[:]})
	if got := waitPkt(t, p.Received()); got != pkt2 {
		t.Fatalf("received %v after reconnect, want %v", got, pkt2)
	}
}

func TestPort_RunStopsOnContextCancel(t *testing.T) {
	open := func() (io.ReadWriteCloser, error) {
		return nil, errors.New("never available")
	}
	p := NewPort(PortConfig{Open: open, Backoff: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}

	// Received() is closed once Run returns.
	if _, ok := <-p.Received(); ok {
		t.Fatal("Received() channel should be closed after Run returns")
	}
}

func waitConn(t *testing.T, ch <-chan net.Conn) net.Conn {
	t.Helper()
	select {
	case c := <-ch:
		return c
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a connection")
		return nil
	}
}

func waitPkt(t *testing.T, ch <-chan [PacketLen]byte) [PacketLen]byte {
	t.Helper()
	select {
	case p := <-ch:
		return p
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a packet")
		return [PacketLen]byte{}
	}
}
