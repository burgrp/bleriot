// Package mcpdongle implements radio.Dongle over an MCP2210 USB-to-SPI bridge
// driving a PAN211x in BLE LongRange mode. It is the host-resident ("dumb
// dongle") transport: the PAN211x register sequence for every packet runs on the
// host, so each register access costs a USB-HID round trip. A future smart dongle
// (PAN211x driven by an on-board MCU, exposing a framed USB packet protocol)
// would be a separate radio.Dongle implementation with the same interface.
package mcpdongle

import (
	"sync"
	"time"

	"github.com/burgrp/tinygo-drivers/pan211x"

	"github.com/burgrp/bleriot/lib/shared/protocol"

	"github.com/burgrp/bleriot/lib/site/mcp2210"
)

// Dongle is one MCP2210 + PAN211x RF endpoint on a single channel. It owns the
// MCP2210 device handle, the radio driver, a lock serialising all USB access,
// and two status LEDs. It satisfies radio.Dongle (Send / Receive / Close).
type Dongle struct {
	dev       *mcp2210.Device
	registers *registers
	driver    *pan211x.DriverBLELongRange
	// guard is the reply turnaround delay this dongle asks nodes to honour, fixed
	// at Open from the spreading factor (see ReplyGuard).
	guard time.Duration
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
	registers := newRegisters(dev)
	driver := pan211x.NewDriverBLELongRange(registers)
	if err := driver.Init(pan211x.ConfigBLELongRange{
		PayloadLen:      protocol.PacketLen,
		SerialInterface: pan211x.SerialInterfaceSPI3W,
		SpreadFactor:    spreadFactor,
	}); err != nil {
		dev.Close()
		return nil, err
	}
	if err := driver.SetChannelRF(channel, channel); err != nil {
		dev.Close()
		return nil, err
	}
	if err := driver.EnableRxAddress(0, rxAddr); err != nil {
		dev.Close()
		return nil, err
	}
	d := &Dongle{dev: dev, registers: registers, driver: driver, guard: replyGuard(spreadFactor)}
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
	err := d.driver.Send(dst, payload)
	if transportErr := d.registers.takeError(); err == nil {
		err = transportErr
	}
	return err
}

// Receive polls for one packet, lighting the green activity LED when one
// arrives. Transport errors are available to supervisors via ReceiveWithError.
func (d *Dongle) Receive(buf []byte) (int, bool) {
	n, ok, _ := d.ReceiveWithError(buf)
	return n, ok
}

// ReceiveWithError polls for one packet and preserves register transport
// errors that the PAN211x driver's two-result Receive API cannot represent.
func (d *Dongle) ReceiveWithError(buf []byte) (int, bool, error) {
	d.mu.Lock()
	n, ok := d.driver.Receive(buf)
	err := d.registers.takeError()
	d.mu.Unlock()
	if ok && err == nil {
		d.green.trigger()
	}
	return n, ok, err
}

// ReplyGuard reports the reply turnaround delay this dongle asks nodes to wait
// before answering a request (lib/README.md §6, GUARD). The MCP2210 is a
// host-resident "dumb" dongle: after transmitting a request it needs several
// USB-HID round trips to switch the PAN211x back to receive, during which a node
// that replied immediately would be missed. The hub puts this delay in every
// request and the node honours it before replying.
func (d *Dongle) ReplyGuard() time.Duration { return d.guard }

// ReplyGuard returns the reply turnaround guard (lib/README.md §6) an MCP2210
// dongle running at the given spreading factor asks nodes to honour. It is a
// device-independent constant, so a caller can learn the guard without an open
// device — e.g. to supervise a not-yet-connected dongle whose guard the engine
// must know before the hardware appears.
func ReplyGuard(sf pan211x.SpreadFactor) time.Duration { return replyGuard(sf) }

// replyGuard returns the reply turnaround guard for a spreading factor.
//
// Timing budget (MCP2210, "dumb" host-resident dongle):
//   - After an on-air transmit the driver's enterRX() issues three PAN211x
//     register writes (STATE_CFG=STB3, RFIRQFLG=IRQ_ALL, STATE_CFG=RX). On the
//     MCP2210 each register access is at least one USB-HID round trip, frame-
//     bounded at USB full speed, so the transmit→receive switch costs roughly
//     6–11 ms in practice.
//   - 20 ms is an empirical safety value: about 2× the worst observed turnaround,
//     and well under the engine's per-attempt timeout (DefaultTimeout 50 ms, and
//     500 ms in the dongle functional tests).
//   - Once the PAN211x reaches STATE_RX it captures the reply autonomously into
//     its 128-byte FIFO, so only *reaching* RX before the reply starts matters;
//     host poll latency afterwards does not.
//
// The guard is essentially independent of the spreading factor. On-air time does
// differ by factor (~1.4 ms S8 vs ~0.64 ms S2 for a 13-byte packet) but it
// cancels out: the request's on-air time anchors both endpoints to the same
// end-of-transmission instant, and all that remains is the SF-independent enterRX
// USB cost. The value is kept per-factor only as a tuning hook for a future smart
// (MCU-local) dongle, whose microsecond turnaround would let on-air time dominate.
func replyGuard(sf pan211x.SpreadFactor) time.Duration {
	switch sf {
	case pan211x.SpreadFactorS2:
		return 20 * time.Millisecond
	default: // SpreadFactorS8
		return 20 * time.Millisecond
	}
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
