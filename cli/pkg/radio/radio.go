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

// dongle is the shared transport over one MCP2210 + PAN211x: the device handle,
// the radio driver, a lock serialising all USB access, and the two status LEDs.
// Both the hub-side Radio and the node-side NodeRadio embed it.
type dongle struct {
	dev    *mcp2210.Device
	driver *pan211x.DriverBLELongRange
	// mu serialises USB access: transmit, the receive poll, and LED updates all
	// share the one HID device.
	mu    sync.Mutex
	red   *led // lit while transmitting
	green *led // lit when a packet arrives
}

// openDongle configures the MCP2210, brings up its PAN211x for BLE LongRange on
// the given channel, and sets the pipe-0 receive address. rxAddr is the address
// this endpoint listens on (the hub address on the hub side, the node address on
// the node side).
func openDongle(dev *mcp2210.Device, channel uint8, rxAddr pan211x.AddressBLE) (*dongle, error) {
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
	if err := driver.EnableRxAddress(0, rxAddr); err != nil {
		return nil, err
	}
	d := &dongle{dev: dev, driver: driver}
	d.red = newLED(d.setGPIO, ledRedPin)
	d.green = newLED(d.setGPIO, ledGreenPin)
	return d, nil
}

// setGPIO drives a dongle GPIO pin under the shared USB lock. LED errors are
// non-fatal: a missed status-LED update must never break radio traffic.
func (d *dongle) setGPIO(pin uint8, on bool) {
	d.mu.Lock()
	_ = d.dev.SetGPIO(pin, on)
	d.mu.Unlock()
}

// send transmits payload to dst and lights the red activity LED.
func (d *dongle) send(dst pan211x.AddressBLE, payload []byte) error {
	d.red.trigger()
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.driver.Send(dst, payload)
}

// receive polls for one packet, lighting the green activity LED when one
// arrives. It never blocks.
func (d *dongle) receive(buf []byte) (int, bool) {
	d.mu.Lock()
	n, ok := d.driver.Receive(buf)
	d.mu.Unlock()
	if ok {
		d.green.trigger()
	}
	return n, ok
}

// stopLEDs turns both LEDs off and cancels their timers.
func (d *dongle) stopLEDs() {
	d.red.stop()
	d.green.stop()
}

// Radio drives a single PAN211x on one RF channel through an MCP2210 dongle and
// satisfies the hub engine's Radio interface (Send / Received).
type Radio struct {
	*dongle
	in chan [protocol.PacketLen]byte
}

// New initialises the dongle's radio for BLE LongRange on the given channel and
// receive address (the hub address, so node replies are received), then starts
// a receive loop that runs until ctx is cancelled. The caller retains ownership
// of dev and is responsible for closing it after ctx is done.
func New(ctx context.Context, dev *mcp2210.Device, channel uint8, hubAddr pan211x.AddressBLE) (*Radio, error) {
	d, err := openDongle(dev, channel, hubAddr)
	if err != nil {
		return nil, err
	}
	r := &Radio{
		dongle: d,
		in:     make(chan [protocol.PacketLen]byte, 16),
	}
	go r.recvLoop(ctx)
	return r, nil
}

// Send transmits payload to dst, then re-enters receive mode. It lights the red
// LED for the duration of the transmit-activity window.
func (r *Radio) Send(dst pan211x.AddressBLE, payload []byte) error {
	return r.send(dst, payload)
}

// Received returns the channel of inbound packets. It is closed once ctx (passed
// to New) is cancelled.
func (r *Radio) Received() <-chan [protocol.PacketLen]byte {
	return r.in
}

// recvLoop polls the radio for received packets and forwards complete ones.
func (r *Radio) recvLoop(ctx context.Context) {
	defer close(r.in)
	defer r.stopLEDs()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	buf := make([]byte, protocol.PacketLen)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, ok := r.receive(buf)
			if !ok || n != protocol.PacketLen {
				continue
			}
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
