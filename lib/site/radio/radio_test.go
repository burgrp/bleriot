package radio

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/burgrp/bleriot/lib/shared/protocol"
)

type dongleSend struct {
	dst     [4]byte
	payload []byte
}

type dongleReceive struct {
	payload []byte
	n       int
	ok      bool
}

type adapterDongle struct {
	sends     chan dongleSend
	receives  chan dongleReceive
	consumed  chan struct{}
	guard     time.Duration
	sendError error

	mu         sync.Mutex
	closeCount int
}

func newAdapterDongle() *adapterDongle {
	return &adapterDongle{
		sends:    make(chan dongleSend, 1),
		receives: make(chan dongleReceive, 32),
		consumed: make(chan struct{}, 32),
	}
}

func (dongle *adapterDongle) Send(dst [4]byte, payload []byte) error {
	dongle.sends <- dongleSend{dst: dst, payload: append([]byte(nil), payload...)}
	return dongle.sendError
}

func (dongle *adapterDongle) Receive(buf []byte) (int, bool) {
	select {
	case result := <-dongle.receives:
		copy(buf, result.payload)
		dongle.consumed <- struct{}{}
		return result.n, result.ok
	default:
		return 0, false
	}
}

func (dongle *adapterDongle) ReplyGuard() time.Duration { return dongle.guard }

func (dongle *adapterDongle) Close() error {
	dongle.mu.Lock()
	dongle.closeCount++
	dongle.mu.Unlock()
	return nil
}

func (dongle *adapterDongle) closes() int {
	dongle.mu.Lock()
	defer dongle.mu.Unlock()
	return dongle.closeCount
}

func TestRadioForwardsDongleOperations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	dongle := newAdapterDongle()
	dongle.guard = 17 * time.Millisecond
	dongle.sendError = errors.New("send failed")
	radio := New(ctx, dongle)

	dst := [4]byte{1, 2, 3, 4}
	payload := []byte{5, 6, 7}
	if err := radio.Send(dst, payload); !errors.Is(err, dongle.sendError) {
		t.Fatalf("Send error = %v, want %v", err, dongle.sendError)
	}
	payload[0] = 0xff
	sent := <-dongle.sends
	if sent.dst != dst || !bytes.Equal(sent.payload, []byte{5, 6, 7}) {
		t.Fatalf("dongle Send = dst %x payload %x", sent.dst, sent.payload)
	}
	if got := radio.ReplyGuard(); got != dongle.guard {
		t.Fatalf("ReplyGuard = %v, want %v", got, dongle.guard)
	}

	short := bytes.Repeat([]byte{0xaa}, protocol.PacketLen-1)
	want := [protocol.PacketLen]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	dongle.receives <- dongleReceive{payload: short, n: len(short), ok: true}
	dongle.receives <- dongleReceive{payload: want[:], n: len(want), ok: true}
	if got := <-radio.Received(); got != want {
		t.Fatalf("Received = %x, want %x", got, want)
	}

	cancel()
	<-radio.Done()
	if _, ok := <-radio.Received(); ok {
		t.Fatal("Received remained open after Done")
	}
	if got := dongle.closes(); got != 1 {
		t.Fatalf("dongle Close calls = %d, want 1", got)
	}
}

func TestRadioCancellationUnblocksFullOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	dongle := newAdapterDongle()
	radio := New(ctx, dongle)
	packet := bytes.Repeat([]byte{0x5a}, protocol.PacketLen)

	for range cap(radio.in) + 1 {
		dongle.receives <- dongleReceive{payload: packet, n: len(packet), ok: true}
		<-dongle.consumed
	}
	cancel()
	<-radio.Done()

	for range radio.Received() {
	}
	if got := dongle.closes(); got != 1 {
		t.Fatalf("dongle Close calls = %d, want 1", got)
	}
}

func TestNodeRadioForwardsDongleOperations(t *testing.T) {
	dongle := newAdapterDongle()
	dongle.sendError = errors.New("send failed")
	radio := NewNode(dongle)
	dst := [4]byte{9, 8, 7, 6}
	payload := []byte{1, 3, 5, 7}
	if err := radio.Send(dst, payload); !errors.Is(err, dongle.sendError) {
		t.Fatalf("Send error = %v, want %v", err, dongle.sendError)
	}
	sent := <-dongle.sends
	if sent.dst != dst || !bytes.Equal(sent.payload, payload) {
		t.Fatalf("dongle Send = dst %x payload %x", sent.dst, sent.payload)
	}

	want := []byte{2, 4, 6, 8}
	dongle.receives <- dongleReceive{payload: want, n: len(want), ok: true}
	buf := make([]byte, 8)
	n, ok := radio.Receive(buf)
	if !ok || n != len(want) || !bytes.Equal(buf[:n], want) {
		t.Fatalf("Receive = n %d ok %v payload %x", n, ok, buf[:n])
	}
	if err := radio.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := dongle.closes(); got != 1 {
		t.Fatalf("dongle Close calls = %d, want 1", got)
	}
}
