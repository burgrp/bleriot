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
	"math"
	"runtime"
	"time"
	"unsafe"

	"cli/pkg/config"
	"protocol"
	"protocol/node"
	"thermostat"

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

	// Application I/O. These are the example's BOB wiring choices.
	pinRelay = machine.PA1 // heating relay (active-high)
)

// Provisioning page location in flash for the PY32F030 (see
// cli/pkg/inventory: page 0x0800F800). pageBytes is a fixed read window large
// enough for the header (30) + thermostat Config (8) + CRC (4); config.Unmarshal
// tolerates the trailing slack.
const (
	pageAddr  = 0x0800F800
	pageBytes = 64
)

// sampleInterval is how often the temperature sensor is read and the control
// loop re-evaluated.
const sampleInterval = time.Second

func main() {
	println("BleRiot thermostat starting...")

	pinLedGreen.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinLedRed.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinRelay.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinRelay.Low()

	// Identity and configuration come from the provisioning page in flash. Decode
	// (not Unmarshal) keeps the firmware small by avoiding reflection/fmt.
	pageData := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(pageAddr))), pageBytes)
	header, cfgBytes, err := config.Decode(pageData)
	if err != nil {
		if config.IsUnprovisioned(err) {
			// Fresh device: nothing to run until it is provisioned over SWD.
			haltBlink("unprovisioned", 1000*time.Millisecond)
		}
		haltBlink("bad page: "+err.Error(), 100*time.Millisecond)
	}
	cfg := decodeConfig(cfgBytes)
	println("Provisioned: channel", int(header.Channel), "spreadFactor", int(header.SpreadFactor))

	// Radio: PAN211x in BLE LongRange mode, then apply the page's channel and
	// receive address once (the runtime never reconfigures the radio).
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

	// Build the device and the runtime around it.
	dev := thermostat.New(cfg)
	rt, err := node.New(radio, header.Address, header.Key, dev)
	must(err)
	dev.Attach(rt)

	run(rt, dev)
}

// run is the single cooperative loop: poll the radio for one request, then —
// once per sampleInterval — read the sensor, update the device, and drive the
// relay from the resulting heating decision. It never returns.
func run(rt *node.Node, dev *thermostat.Device) {
	next := time.Now()
	for {
		rt.Poll()

		if now := time.Now(); !now.Before(next) {
			next = now.Add(sampleInterval)

			dev.UpdateTemperature(readTemperature())
			pinRelay.Set(dev.Heating())
			pinLedGreen.Set(!pinLedGreen.Get()) // heartbeat
		}

		runtime.Gosched()
	}
}

// decodeConfig reads the thermostat Config from the page's config bytes. The
// layout matches the Config struct (config.Marshal encodes fields in declaration
// order, little-endian): MinTemp then MaxTemp, each a float32.
func decodeConfig(b []byte) thermostat.Config {
	if len(b) < 8 {
		return thermostat.Config{}
	}
	return thermostat.Config{
		MinTemp: math.Float32frombits(binary.LittleEndian.Uint32(b[0:4])),
		MaxTemp: math.Float32frombits(binary.LittleEndian.Uint32(b[4:8])),
	}
}

// readTemperature returns the current temperature in wire units (hundredths of a
// degree Celsius).
//
// The PY32F030 machine package exposes no ADC, so this example uses a fixed
// placeholder reading. A real BOB build replaces this with its sensor driver —
// e.g. an I2C thermometer read over github.com/burgrp/tinygo-drivers/bb/i2c —
// and returns the measured value here.
func readTemperature() int32 {
	return 2100 // 21.00 °C placeholder
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
		time.Sleep(period)
		pinLedRed.Low()
		time.Sleep(period)
	}
}
