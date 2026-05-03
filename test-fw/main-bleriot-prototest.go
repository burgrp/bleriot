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

type Role bool

const (
	RoleHub  Role = false
	RoleNode Role = true
)

var roleNames = map[Role]string{
	RoleHub:  "Hub",
	RoleNode: "Node",
}

type Device struct {
	role       Role
	myAddr     [4]byte
	peerAddr   [4]byte
	pinSpiSck  machine.Pin
	pinSpiData machine.Pin
	pinSpiCsn  machine.Pin
	led        machine.Pin
}

func (d *Device) start() {
	println("Initializing", roleNames[d.role])

	d.led.High()
	time.Sleep(500 * time.Millisecond)
	d.led.Low()

	radio := pan211x.NewDriverBLELongRange(pan211x.NewRegistersSPI(spi.NewMaster(d.pinSpiSck, d.pinSpiData), d.pinSpiCsn))

	d.must(radio.Init(pan211x.ConfigBLELongRange{
		PayloadLen:      bleriot.PacketLen,
		SerialInterface: pan211x.SerialInterfaceSPI3W,
		SpreadFactor:    pan211x.SpreadFactorS8,
	}))

	d.must(radio.SetChannel(11))
	d.must(radio.EnableRxAddress(0, d.myAddr))

	println(roleNames[d.role], "initialized")

	nextPacket := time.Now()

	var buf [bleriot.PacketLen]byte
	for {
		n, ok := radio.Receive(buf[:])
		if ok {
			println(roleNames[d.role], "received", n, "bytes:", buf[:n])
			d.led.Set(!d.led.Get())
		}

		if time.Now().After(nextPacket) {
			err := radio.Send(d.peerAddr, []byte{0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF, 0xDE, 0xAD, 0xBE, 0xEF})
			if err != nil {
				println(roleNames[d.role], "transmit error:", err.Error())
			} else {
				println(roleNames[d.role], "transmitted packet")
			}
			nextPacket = time.Now().Add(500 * time.Millisecond)
		}

		runtime.Gosched()
	}

}

func (d *Device) must(err error) {
	if err != nil {
		panic(roleNames[d.role] + ": " + err.Error())
	}
}

func main() {
	println("BleRiot proto-test starting...")

	pinLedGreen.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinLedRed.Configure(machine.PinConfig{Mode: machine.PinOutput})

	hubAddr := [4]byte{0x00, 0x00, 0x00, 0x00}
	nodeAddr := [4]byte{0x00, 0x00, 0x00, 0x01}

	go (&Device{
		role:       RoleHub,
		myAddr:     hubAddr,
		peerAddr:   nodeAddr,
		pinSpiSck:  machine.PA9,
		pinSpiData: machine.PA7,
		pinSpiCsn:  machine.PA10,
		led:        pinLedRed,
	}).start()

	go (&Device{
		role:       RoleNode,
		myAddr:     nodeAddr,
		peerAddr:   hubAddr,
		pinSpiSck:  machine.PB4,
		pinSpiData: machine.PB3,
		pinSpiCsn:  machine.PB5,
		led:        pinLedGreen,
	}).start()

	for {
		printMemoryStats()
		time.Sleep(5000 * time.Millisecond)
	}

}

func printMemoryStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	println("RAM:", m.HeapAlloc, "/", m.HeapSys, "bytes, Mallocs:", m.Mallocs)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
