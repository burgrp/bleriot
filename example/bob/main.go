//go:build tinygo

// Command (firmware) main is the BleRiot thermostat node for the BOB breakout
// board (PY32F030 + PAN211x). It is a full protocol node: it owns the radio,
// reads its identity and config from the provisioning page in flash, and runs
// the BleRiot runtime (protocol/node) over a thermostat device (the example
// package).
//
// On boot it:
//   - reads the provisioning page (address, XTEA key, channel, Config) from
//     flash and halts with a blink pattern if the device is unprovisioned;
//   - initialises the PAN211x radio in BLE LongRange mode and applies the
//     channel and receive address from the page;
//   - builds the thermostat device and the node runtime, then loops forever:
//     it polls the radio for GET/SET/WATCH requests, samples the temperature
//     sensor periodically, and drives the heating relay from the device's
//     decision.
//
// All XTEA crypto and register dispatch live in protocol/node; this file is only
// hardware wiring. Debug logging uses println() over SEGGER RTT.
package main

import (
	"encoding/binary"
	"machine"
	"runtime"
	"sync/atomic"
	"time"
	"unsafe"

	"bob/spec"
	"protocol"
	"protocol/node"
	"site/config"

	"github.com/burgrp/tinygo-drivers/bb/spi"
	"github.com/burgrp/tinygo-drivers/pan211x"
)

const (
	pinLedRed   = machine.PB0 // lit on fatal fault (blink pattern)
	pinLedGreen = machine.PB1 // heartbeat

	// PAN211x over 3-wire SPI.
	pinSpiSck  = machine.PA9  // SCK  → PAN211x pin 2
	pinSpiData = machine.PA7  // DATA → PAN211x pin 3, bidirectional
	pinSpiCsn  = machine.PA10 // CSN  → PAN211x pin 1, active-low
)

// sampleInterval is how often the temperature sensor is read and the control
// loop re-evaluated.
const sampleInterval = time.Second

var gpioPins = [7]machine.Pin{
	machine.PA0,
	machine.PA1,
	machine.PA2,
	machine.PA3,
	machine.PA4,
	machine.PA5,
	machine.PA6,
}

func main() {
	println("BleRiot thermostat starting...")

	pinLedGreen.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinLedRed.Configure(machine.PinConfig{Mode: machine.PinOutput})
	for _, pin := range gpioPins {
		pin.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})
	}

	pinLedGreen.High()
	pinLedRed.Low()
	time.Sleep(500 * time.Millisecond)
	pinLedGreen.Low()
	pinLedRed.High()

	pageData := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(spec.Chip.PageAddr))), spec.Chip.PageBytes)
	header, cfgBytes, err := config.Decode(pageData)
	if err != nil {
		if config.IsUnprovisioned(err) {
			haltBlink("unprovisioned", 1000*time.Millisecond)
		}
		haltBlink("bad page: "+err.Error(), 100*time.Millisecond)
	}
	cfg := spec.Config{
		DefaultRedPeriod:   binary.LittleEndian.Uint32(cfgBytes[0:4]),
		DefaultGreenPeriod: binary.LittleEndian.Uint32(cfgBytes[4:8]),
	}

	println("Provisioned: channel", int(header.Channel), "spreadFactor", int(header.SpreadFactor))

	radio := pan211x.NewDriverBLELongRange(
		pan211x.NewRegistersSPI(spi.NewMaster(pinSpiSck, pinSpiData), pinSpiCsn))
	must(radio.Init(pan211x.ConfigBLELongRange{
		PayloadLen:      protocol.PacketLen,
		SerialInterface: pan211x.SerialInterfaceSPI3W,
		SpreadFactor:    pan211x.SpreadFactor(header.SpreadFactor),
	}))
	must(radio.SetChannel(header.Channel))
	must(radio.EnableRxAddress(0, header.Address))
	println("Radio initialized")

	println("Device config: defaultRedPeriod", cfg.DefaultRedPeriod, "defaultGreenPeriod", cfg.DefaultGreenPeriod)

	device := &Device{}
	device.redPeriod.Store(int32(cfg.DefaultRedPeriod))
	device.greenPeriod.Store(int32(cfg.DefaultGreenPeriod))

	node, err := node.New(radio, header.Address, header.Key, device)
	must(err)
	device.node = node

	go device.ledLoop(pinLedRed, &device.redPeriod)
	go device.ledLoop(pinLedGreen, &device.greenPeriod)

	pins := device.readPins()
	device.pins.Store(pins)
	for {
		node.Poll()
		runtime.Gosched()
		p := device.readPins()
		if p != pins {
			pins = p
			node.Notify(spec.RegGpio, pins, false)
			device.pins.Store(pins)
		}
	}
}

type Device struct {
	redPeriod   atomic.Int32
	greenPeriod atomic.Int32
	pins        atomic.Int32
	node        *node.Node
}

func (d *Device) readPins() int32 {
	var bits int32
	for i, pin := range gpioPins {
		if pin.Get() {
			bits |= 1 << i
		}
	}
	return bits
}

func (d *Device) Read(tag uint16) (value int32, null bool) {

	switch tag {
	case spec.RegLedRed:
		return d.redPeriod.Load(), false
	case spec.RegLedGreen:
		return d.greenPeriod.Load(), false
	case spec.RegGpio:
		return d.readPins(), false
	default:
		// unknown tag: report null
	}

	return 0, true
}

func (d *Device) Write(tag uint16, value int32, null bool) {

	switch tag {
	case spec.RegLedRed:
		d.redPeriod.Store(int32(value))
	case spec.RegLedGreen:
		d.greenPeriod.Store(int32(value))
	default:
		// unknown tag: ignore
	}
}

func (d *Device) ledLoop(pin machine.Pin, period *atomic.Int32) {
	for {
		v := period.Load()
		switch {
		case v == 0:
			pin.Low()
			time.Sleep(100 * time.Millisecond)
		case v == 1:
			pin.High()
			time.Sleep(100 * time.Millisecond)
		default:
			p := time.Duration(v) * time.Millisecond
			pin.High()
			time.Sleep(p)
			pin.Low()
			time.Sleep(p)
		}
	}
}

// must halts with a visible blink pattern if a one-time setup step fails. There
// is no recovery from a radio that will not initialise.
func must(err error) {
	if err == nil {
		return
	}
	haltBlink("fatal: "+err.Error(), 100*time.Millisecond)
}

// haltBlink logs msg once and blinks the red LED forever; the device cannot make
// progress (unprovisioned, bad page, or a failed peripheral).
func haltBlink(msg string, period time.Duration) {
	println(msg)
	for {
		pinLedRed.High()
		pinLedGreen.Low()
		time.Sleep(period)
		pinLedRed.Low()
		pinLedGreen.High()
		time.Sleep(period)
	}
}
