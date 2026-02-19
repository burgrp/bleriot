package main

import (
	"device/py32"
	"machine"
	"runtime"
	"test-fw/i2c"
	"test-fw/pan211x"
	"time"
)

const (
	pinUartTx   = machine.PB6
	pinUartRx   = machine.PB7
	pinLedRed   = machine.PB0
	pinLedGreen = machine.PB1
	pinI2cSDA   = machine.PA7
	pinI2cSCL   = machine.PA9
)

var send = []byte("Hello, PAN211x!")

func main() {
	machine.ConfigureUARTPin(pinUartTx, 0) // TX
	machine.ConfigureUARTPin(pinUartRx, 0) // RX

	pinLedGreen.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinLedRed.Configure(machine.PinConfig{Mode: machine.PinOutput})

	pinI2cSDA.Configure(machine.PinConfig{Mode: machine.PinAlternate})
	pinI2cSDA.SetAltFunc(12)
	pinI2cSCL.Configure(machine.PinConfig{Mode: machine.PinAlternate})
	pinI2cSCL.SetAltFunc(6)
	py32.RCC.APBENR1.SetBits(py32.RCC_APBENR1_I2CEN)

	i2cMaster := i2c.NewMaster(py32.I2C, 24_000_000, 100_000)

	regs := pan211x.NewRegistersI2C(i2cMaster)

	pan := pan211x.NewDriver(regs)

	for {

		pan.Send(send)

		//regs.Read(0x0F)

		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		println("Alloc: ", stats.Alloc, " TotalAlloc: ", stats.TotalAlloc, " Sys: ", stats.Sys)

		time.Sleep(500 * time.Millisecond)
	}

}

// func hex32(v uint32) string {
// 	digits := "0123456789ABCDEF"
// 	return "0x" + string([]byte{
// 		digits[v>>28&0xf],
// 		digits[v>>24&0xf],
// 		digits[v>>20&0xf],
// 		digits[v>>16&0xf],
// 		digits[v>>12&0xf],
// 		digits[v>>8&0xf],
// 		digits[v>>4&0xf],
// 		digits[v&0xf],
// 	})
// }
