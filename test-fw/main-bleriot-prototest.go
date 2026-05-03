package main

import (
	"machine"
	"runtime"
	"test-fw/bleriot"
	"time"

	"github.com/burgrp/tinygo-drivers/bb/spi"
	"github.com/burgrp/tinygo-drivers/pan211x"
)

const (
	pinLedRed   = machine.PB0
	pinLedGreen = machine.PB1

	// pinSpiSck  = machine.PA9  // SCK  → PAN211x pin 2
	// pinSpiData = machine.PA7  // DATA → PAN211x pin 3, bidirectional
	// pinSpiCsn  = machine.PA10 // CSN  → PAN211x pin 1, active-low
)

type Device struct {
	role       string
	myAddr     [4]byte
	peerAddr   [4]byte
	pinSpiSck  machine.Pin
	pinSpiData machine.Pin
	pinSpiCsn  machine.Pin
	led        machine.Pin
}

func (d *Device) start() {
	println("Initializing", d.role)

	d.led.High()
	time.Sleep(500 * time.Millisecond)
	d.led.Low()

	driver := pan211x.NewDriverBLELongRange(pan211x.NewRegistersSPI(spi.NewMaster(d.pinSpiSck, d.pinSpiData), d.pinSpiCsn))

	d.must(driver.Init(pan211x.ConfigBLELongRange{
		PayloadLen:      bleriot.PacketLen,
		SerialInterface: pan211x.SerialInterfaceSPI3W,
		SpreadFactor:    pan211x.SpreadFactorS8,
	}))

	d.must(driver.SetChannel(11))
	d.must(driver.EnableRxAddress(0, d.myAddr))

	println(d.role, "initialized")

	nextPacket := time.Now().Add(1 * time.Second)

	var buf [bleriot.PacketLen]byte
	for {
		n, ok := driver.Receive(buf[:])
		if ok {
			println(d.role, "received", n, "bytes:", buf[:n])
			d.led.Set(!d.led.Get())
		}

		if time.Now().After(nextPacket) {
			err := driver.Send(d.peerAddr, []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF})
			if err != nil {
				println(d.role, "transmit error:", err.Error())
			} else {
				println(d.role, "transmitted packet")
			}
			nextPacket = time.Now().Add(1 * time.Second)
		}

		runtime.Gosched()
	}
}

func (d *Device) must(err error) {
	if err != nil {
		panic(d.role + ": " + err.Error())
	}
}

func main() {
	println("BleRiot proto-test starting...")

	pinLedGreen.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinLedRed.Configure(machine.PinConfig{Mode: machine.PinOutput})

	hubAddr := [4]byte{0x00, 0x00, 0x00, 0x00}
	nodeAddr := [4]byte{0x00, 0x00, 0x00, 0x01}

	hub := Device{
		role:       "hub",
		myAddr:     hubAddr,
		peerAddr:   nodeAddr,
		pinSpiSck:  machine.PA9,
		pinSpiData: machine.PA7,
		pinSpiCsn:  machine.PA10,
		led:        pinLedRed,
	}

	node := Device{
		role:       "node",
		myAddr:     nodeAddr,
		peerAddr:   hubAddr,
		pinSpiSck:  machine.PB4,
		pinSpiData: machine.PB3,
		pinSpiCsn:  machine.PB5,
		led:        pinLedGreen,
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	println("-------------------")
	println("Alloc:", m.Alloc)
	println("Sys:", m.Sys)
	println("Mallocs:", m.Mallocs)

	go hub.start()

	runtime.ReadMemStats(&m)
	println("-------------------")
	println("Alloc:", m.Alloc)
	println("Sys:", m.Sys)
	println("Mallocs:", m.Mallocs)

	go node.start()

	for {
		time.Sleep(500 * time.Millisecond)

		runtime.ReadMemStats(&m)
		println("-------------------")
		println("Alloc:", m.Alloc)
		println("Sys:", m.Sys)
		println("Mallocs:", m.Mallocs)
	}

}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
