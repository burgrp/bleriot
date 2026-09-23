//go:build tinygo

// Command (firmware) main is the BleRiot node for the BOB breakout
// board (PY32F030 + PAN211x). It is a full protocol node: it owns the radio and
// runs the BleRiot runtime (lib/node) over the bob device (the example
// package).
//
// The device's identity (RF address, XTEA key, channel, spread factor) and its
// config are compiled into the program image rather than loaded from a separate
// provisioning flash page. The host "bleriot make" command generates a tiny
// main() that calls bleriotMain (this file) with a node.Provisioning value and a
// spec.Config. That generated main lives in main_gen.go (gitignored, written by
// "bleriot make" before each build).
//
// On boot bleriotMain:
//   - initialises the PAN211x radio in BLE LongRange mode and applies the
//     channel and receive address from the provisioning;
//   - builds the bob device and the node runtime, then loops forever:
//     it polls the radio for GET/SET requests, drives the green LED from its
//     period register, and uses the red LED for offline status. GPIO input pins
//     are sampled for each GET.
//
// All XTEA crypto and register dispatch live in lib/node; this file is only
// hardware wiring. Debug logging uses println() over SEGGER RTT.
package main

import (
	"machine"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/burgrp/bleriot/example/bob/spec"
	"github.com/burgrp/bleriot/lib/node"

	"github.com/burgrp/bleriot/lib/node/pan211x"
)

const (
	pinLedRed   = machine.PB0 // red output/status LED
	pinLedGreen = machine.PB1 // green output/status LED

	// PAN211x over 3-wire SPI.
	pinSpiSck  = machine.PA9  // SCK  → PAN211x pin 2
	pinSpiData = machine.PA7  // DATA → PAN211x pin 3, bidirectional
	pinSpiCsn  = machine.PA10 // CSN  → PAN211x pin 1, active-low
)

var gpioPins = [7]machine.Pin{
	machine.PA0,
	machine.PA1,
	machine.PA2,
	machine.PA3,
	machine.PA4,
	machine.PA5,
	machine.PA6,
}

// bleriotMain is the firmware entry point the generated main() calls with the
// device's baked-in identity and config. It is hand-written; only the trivial
// main() that supplies prov and cfg is generated (main_gen.go).
func bleriotMain(prov node.Provisioning, cfg spec.Config) {
	println("BleRiot bob starting...")

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

	device := &Device{}

	n, err := pan211x.StartNode(prov, pinSpiSck, pinSpiData, pinSpiCsn, device)
	if err != nil {
		haltBlink("failed to start node: "+err.Error(), 100*time.Millisecond)
	}

	println("Device config: defaultLedPeriod", cfg.DefaultLedPeriod)

	device.ledPeriod.Store(int32(cfg.DefaultLedPeriod))

	go device.ledLoop(pinLedGreen, &device.ledPeriod)

	// go memstat()

	for {
		online, statusLED, changed := n.PollWithStatus()
		if changed {
			if redLEDOn(online, statusLED) {
				pinLedRed.High()
			} else {
				pinLedRed.Low()
			}
		}
		runtime.Gosched()
	}

}

// func memstat() {
// 	for {
// 		mem := runtime.MemStats{}
// 		runtime.ReadMemStats(&mem)
// 		println("mem: alloc", mem.Alloc, "sys", mem.Sys, "alloc", mem.HeapAlloc)
// 		time.Sleep(1 * time.Second)
// 	}
// }

type Device struct {
	ledPeriod atomic.Int32
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
	case spec.RegLed:
		return d.ledPeriod.Load(), false
	case spec.RegGpio:
		return d.readPins(), false
	default:
		// unknown tag: report null
	}

	return 0, true
}

func (d *Device) Write(tag uint16, value int32, null bool) {
	switch tag {
	case spec.RegLed:
		writePeriod(&d.ledPeriod, value, null)
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

// haltBlink logs msg once and blinks the red LED forever; the device cannot make
// progress (wrong device or a failed peripheral).
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

func writePeriod(period *atomic.Int32, value int32, null bool) {
	if null || value < 0 {
		value = 0
	}
	period.Store(value)
}
