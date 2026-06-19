// Package mcpdongle implements radio.Dongle over an MCP2210 USB-to-SPI bridge
// driving a PAN211x in BLE LongRange mode. It is the host-resident ("dumb
// dongle") transport: the PAN211x register sequence for every packet runs on the
// host, so each register access costs a USB-HID round trip. A future smart dongle
// (PAN211x driven by an on-board MCU, exposing a framed USB packet protocol)
// would be a separate radio.Dongle implementation with the same interface.
package mcpdongle

import (
	"sync"

	"github.com/burgrp/tinygo-drivers/pan211x"

	"protocol"

	"site/mcp2210"
)

// Dongle is one MCP2210 + PAN211x RF endpoint on a single channel. It owns the
// MCP2210 device handle, the radio driver, a lock serialising all USB access,
// and two status LEDs. It satisfies radio.Dongle (Send / Receive / Close).
type Dongle struct {
	dev    *mcp2210.Device
	driver *pan211x.DriverBLELongRange
	// mu serialises USB access: transmit, the receive poll, and LED updates all
	// share the one HID device.
	mu     sync.Mutex
	closed bool // set under mu by Close; suppresses further USB commands
	red    *led // lit while transmitting
	green  *led // lit when a packet arrives
}

// Open configures the MCP2210, brings up its PAN211x for BLE LongRange on the
// given channel and spreading factor, and sets the pipe-0 receive address (the
// address this endpoint listens on: the hub address on the hub side, the node
// address on the node side). The spreading factor must match the nodes this
// dongle talks to. Open takes ownership of dev: Close closes it, and a failed
// Open closes it before returning.
func Open(dev *mcp2210.Device, channel uint8, spreadFactor pan211x.SpreadFactor, rxAddr [4]byte) (*Dongle, error) {
	if err := dev.Configure(mcp2210.DefaultSPIConfig, ledRedPin, ledGreenPin); err != nil {
		dev.Close()
		return nil, err
	}
	driver := pan211x.NewDriverBLELongRange(newRegisters(dev))
	if err := driver.Init(pan211x.ConfigBLELongRange{
		PayloadLen:      protocol.PacketLen,
		SerialInterface: pan211x.SerialInterfaceSPI3W,
		SpreadFactor:    spreadFactor,
	}); err != nil {
		dev.Close()
		return nil, err
	}
	if err := driver.SetChannel(channel); err != nil {
		dev.Close()
		return nil, err
	}
	if err := driver.EnableRxAddress(0, rxAddr); err != nil {
		dev.Close()
		return nil, err
	}
	d := &Dongle{dev: dev, driver: driver}
	d.red = newLED(d.setGPIO, ledRedPin)
	d.green = newLED(d.setGPIO, ledGreenPin)
	return d, nil
}

// setGPIO drives a dongle GPIO pin under the shared USB lock. LED errors are
// non-fatal: a missed status-LED update must never break radio traffic. After
// Close it is a no-op, so a late-firing LED auto-off timer can never issue a
// command on (or just after) a closed device and leave a stale HID response
// that would desync the next session.
func (d *Dongle) setGPIO(pin uint8, on bool) {
	d.mu.Lock()
	if !d.closed {
		_ = d.dev.SetGPIO(pin, on)
	}
	d.mu.Unlock()
}

// Send transmits payload to dst and lights the red activity LED.
func (d *Dongle) Send(dst [4]byte, payload []byte) error {
	d.red.trigger()
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.driver.Send(dst, payload)
}

// Receive polls for one packet, lighting the green activity LED when one
// arrives. It never blocks.
func (d *Dongle) Receive(buf []byte) (int, bool) {
	d.mu.Lock()
	n, ok := d.driver.Receive(buf)
	d.mu.Unlock()
	if ok {
		d.green.trigger()
	}
	return n, ok
}

// Close turns both status LEDs off, cancels their timers, and closes the
// underlying MCP2210 device. It stops the LED timers first (issuing the final
// off-writes, whose responses are drained), then takes the USB lock before
// closing: this guarantees no command is mid-flight when the device closes and,
// with the closed flag, that none can start afterwards — preventing a stale HID
// response from desyncing a subsequent Open of the same device.
func (d *Dongle) Close() error {
	d.red.stop()
	d.green.stop()
	d.mu.Lock()
	d.closed = true
	err := d.dev.Close()
	d.mu.Unlock()
	return err
}
