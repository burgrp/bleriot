package main

import (
	"machine"
	"test-fw/pan211x"
	"test-fw/spi"
	"time"
)

const (
	pinLedRed   = machine.PB0
	pinLedGreen = machine.PB1

	pinSpiSck  = machine.PA9  // SCK  → PAN211x pin 2
	pinSpiData = machine.PA7  // DATA → PAN211x pin 3, bidirectional
	pinSpiCsn  = machine.PA10 // CSN  → PAN211x pin 1, active-low
)

// Static random BLE address (bits[47:46]=0b11 in MSB = advA[5] bits[7:6]=0b11).
// Appears as C0:05:04:03:02:01 on the scanner.
var bleAdvA = [6]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0xC0}

// advData: Flags + Complete Local Name "BleRiot".
var bleAdvData = [...]byte{
	0x02, 0x01, 0x06,                               // Flags: LE general discoverable, BR/EDR not supported
	0x08, 0x09, 'B', 'l', 'e', 'R', 'i', 'o', 't', // Complete Local Name: BleRiot
}

func main() {
	println("BleRiot BLE TX starting...")

	machine.ConfigureUARTPin(machine.PB6, 0)
	machine.ConfigureUARTPin(machine.PB7, 0)

	pinLedGreen.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinLedRed.Configure(machine.PinConfig{Mode: machine.PinOutput})

	pinSpiCsn.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinSpiCsn.High()
	spiMaster := spi.NewMaster(pinSpiSck, pinSpiData)
	regs := pan211x.NewRegistersSPI(spiMaster, pinSpiCsn)

	pan := pan211x.NewDriver(regs, pan211x.Config{})

	if err := pan.InitBLE(); err != nil {
		println("init error:", err.Error())
		for {
		}
	}
	println("BLE ready - advertising as C0:05:04:03:02:01 'BleRiot'")

	var count uint32
	for {
		count++
		if err := pan.AdvertiseBLE(bleAdvA, bleAdvData[:]); err != nil {
			println("TX err:", err.Error())
			pinLedRed.High()
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if count%50 == 0 {
			println("ADV count:", count)
			pinLedGreen.Set(!pinLedGreen.Get())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
