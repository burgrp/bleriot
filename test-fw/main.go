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

const (
	nodeAddr pan211x.Address = 0xAA556996
	hubAddr  pan211x.Address = 0x55AA9669
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

	isHub := !pinRoleHub.Get()

	addr := nodeAddr
	if isHub {
		addr = hubAddr
	}

	pan := pan211x.NewDriver(regs, pan211x.Config{
		OwnAddr:    addr,
		RFChannel:  40,
		DataRate:   pan211x.DataRate250kbps,
		PayloadLen: payloadLen,
	})

	if err := pan.Init(); err != nil {
		println("init error:", err.Error())
		for {
		}
	}
	println("Radio OK")
	//pan.DumpState()

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
	dst := nodeAddr.Bytes()

	for {
		counter++
		putU32le(buf[:], counter)

		if err := pan.Send(dst, buf[:]); err != nil {
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
	dst := hubAddr.Bytes()

	for {
		n, ok := pan.Receive(buf[:])
		if !ok || n != payloadLen {
			pinLedRed.Set(!pinLedRed.Get())
			pan.DumpState()
			continue
		}
		v := u32le(buf[:])
		println("RX:", v)
		pinLedGreen.Set(!pinLedGreen.Get())

		if err := pan.Send(dst, buf[:]); err != nil {
			println("TX err:", err.Error())
		}


		//pan.DumpState()
	}
}
