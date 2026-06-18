// Package radio implements the BleRiot host radio over a USB dongle: an MCP2210
// USB-to-SPI bridge driving a PAN211x in BLE LongRange mode. It satisfies the
// engine's Radio interface (Send / Received), so the hub can talk to RF nodes
// with no microcontroller in between — all radio register access happens on the
// host over USB HID.
package radio

import (
	"context"
	"sync"
	"time"

	"github.com/burgrp/tinygo-drivers/pan211x"

	"protocol"

	"cli/pkg/mcp2210"
)

// pollInterval is how often the receive loop checks the radio for an inbound
// packet. The hub is the master in every transaction, so replies arrive well
// within the engine's per-attempt timeout; a short interval keeps latency low
// without a dedicated interrupt line (the dongle has none wired to the host).
const pollInterval = time.Millisecond

// Radio drives a single PAN211x on one RF channel through an MCP2210 dongle.
type Radio struct {
	dev    *mcp2210.Device
	driver *pan211x.DriverBLELongRange
	// mu serialises USB access: Send, the receive poll loop, and LED updates all
	// share the one HID device.
	mu sync.Mutex
	in chan [protocol.PacketLen]byte

	redLED   *led // lit while transmitting
	greenLED *led // lit when a packet arrives
}

// New initialises the dongle's radio for BLE LongRange on the given channel and
// receive address (the hub address, so node replies are received), then starts
// a receive loop that runs until ctx is cancelled. The caller retains ownership
// of dev and is responsible for closing it after ctx is done.
func New(ctx context.Context, dev *mcp2210.Device, channel uint8, hubAddr pan211x.AddressBLE) (*Radio, error) {
	if err := dev.Configure(mcp2210.DefaultSPIConfig, ledRedPin, ledGreenPin); err != nil {
		return nil, err
	}
	driver := pan211x.NewDriverBLELongRange(newRegisters(dev))
	if err := driver.Init(pan211x.ConfigBLELongRange{
		PayloadLen:      protocol.PacketLen,
		SerialInterface: pan211x.SerialInterfaceSPI3W,
		SpreadFactor:    pan211x.SpreadFactorS8,
	}); err != nil {
		return nil, err
	}
	if err := driver.SetChannel(channel); err != nil {
		return nil, err
	}
	if err := driver.EnableRxAddress(0, hubAddr); err != nil {
		return nil, err
	}
	r := &Radio{
		dev:    dev,
		driver: driver,
		in:     make(chan [protocol.PacketLen]byte, 16),
	}
	r.redLED = newLED(r, ledRedPin)
	r.greenLED = newLED(r, ledGreenPin)
	go r.recvLoop(ctx)
	return r, nil
}

// Send transmits payload to dst, then re-enters receive mode. It lights the red
// LED for the duration of the transmit-activity window.
func (r *Radio) Send(dst pan211x.AddressBLE, payload []byte) error {
	r.redLED.trigger()
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.driver.Send(dst, payload)
}

// Received returns the channel of inbound packets. It is closed once ctx (passed
// to New) is cancelled.
func (r *Radio) Received() <-chan [protocol.PacketLen]byte {
	return r.in
}

// recvLoop polls the radio for received packets and forwards complete ones.
func (r *Radio) recvLoop(ctx context.Context) {
	defer close(r.in)
	defer r.redLED.stop()
	defer r.greenLED.stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	buf := make([]byte, protocol.PacketLen)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mu.Lock()
			n, ok := r.driver.Receive(buf)
			r.mu.Unlock()
			if !ok || n != protocol.PacketLen {
				continue
			}
			r.greenLED.trigger()
			var pkt [protocol.PacketLen]byte
			copy(pkt[:], buf)
			select {
			case r.in <- pkt:
			case <-ctx.Done():
				return
			}
		}
	}
}
