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
	pinRoleHub  = machine.PA0 // tie to GND = hub, leave floating = node

	pinSpiSck  = machine.PA9  // SCK  → PAN211x pin 2
	pinSpiData = machine.PA7  // DATA → PAN211x pin 3, bidirectional
	pinSpiCsn  = machine.PA10 // CSN  → PAN211x pin 1, active-low
)

const payloadLen = 4

var (
	nodeAddr pan211x.Address = [5]byte{0xAA, 0x55, 0x69, 0x96, 0x00}
	hubAddr  pan211x.Address = [5]byte{0x55, 0xAA, 0x96, 0x69, 0x00}
)

func main() {
	println("BleRiot starting...")

	machine.ConfigureUARTPin(machine.PB6, 0)
	machine.ConfigureUARTPin(machine.PB7, 0)

	pinLedGreen.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinLedRed.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinRoleHub.Configure(machine.PinConfig{Mode: machine.PinInputPullup})

	pinSpiCsn.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinSpiCsn.High()
	spiMaster := spi.NewMaster(pinSpiSck, pinSpiData)
	regs := pan211x.NewRegistersSPI(spiMaster, pinSpiCsn)

	pan := pan211x.NewDriver(regs)

	must(pan.InitXN297L(pan211x.ConfigXN297L{BitRate: pan211x.BitRate1Mbps, PayloadLen: payloadLen}))

	isHub := !pinRoleHub.Get()

	must(pan.SetChannel(10))

	addr := nodeAddr
	if isHub {
		addr = hubAddr
	}
	must(pan.EnableRxAddress(0, addr))

	println("Radio OK")

	pan.DumpState()

	if isHub {
		println("Role: HUB")
		runHub(pan)
	} else {
		println("Role: NODE")
		runNode(pan)
	}
}

func u32le(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func putU32le(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

func runHub(pan *pan211x.Driver) {
	var counter uint32
	var buf [payloadLen]byte

	for {
		counter++
		putU32le(buf[:], counter)
		if counter%10 == 1 {
			pan.DumpState()
		}

		if err := pan.Send(nodeAddr, buf[:]); err != nil {
			println("TX err:", err.Error())
			time.Sleep(500 * time.Millisecond)
			continue
		}
		println("TX:", counter)

		deadline := time.Now().Add(100 * time.Millisecond)
		got := false
		for time.Now().Before(deadline) {
			n, ok := pan.Receive(buf[:])
			if ok && n == payloadLen {
				println("RX:", u32le(buf[:]))
				got = true
				break
			}
		}
		if !got {
			println("RX timeout")
		}

		time.Sleep(500 * time.Millisecond)
		pinLedRed.Set(!pinLedRed.Get())
	}
}

func runNode(pan *pan211x.Driver) {
	var buf [payloadLen]byte
	var missCount uint32

	for {
		n, ok := pan.Receive(buf[:])
		if !ok || n != payloadLen {
			missCount++
			if missCount%10000 == 0 {
				pinLedRed.Set(!pinLedRed.Get())
				println("----------------------------------------")
				pan.DumpState()
			}
			continue
		}
		missCount = 0
		v := u32le(buf[:])
		println("RX:", v)
		pinLedGreen.Set(!pinLedGreen.Get())

		if err := pan.Send(hubAddr, buf[:]); err != nil {
			println("TX err:", err.Error())
		}

		//pan.DumpState()
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
