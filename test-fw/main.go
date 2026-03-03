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

// BLE ADV_NONCONN_IND payload: AdvA followed by AdvData.
// Header (ADV_NONCONN_IND | TxAdd=1) and Length are auto-inserted by the chip
// via TXHDR0_CFG=0x42 and TxLen register when PKT_EXT_CFG[HDR_LEN_EXIST]=1.
// AdvA DE:AD:BE:EF:00:01 is stored LSB-first as required by the BLE spec.
var send = []byte{
	0x01, 0x00, 0xEF, 0xBE, 0xAD, 0xDE, // AdvA: DE:AD:BE:EF:00:01
	0x02, 0x01, 0x06, // AD Flags: LE General Discoverable, no BR/EDR
	0x04, 0x09, 'B', 'O', 'B', // AD Complete Local Name: "BOB"
}

// printBTAddr prints the advertiser address from the PDU (bytes 0-5, MSB-first display).
func printBTAddr() {
	a := send[0:6]
	const h = "0123456789ABCDEF"
	println("BT addr:", string([]byte{
		h[a[5]>>4], h[a[5]&0xF], ':',
		h[a[4]>>4], h[a[4]&0xF], ':',
		h[a[3]>>4], h[a[3]&0xF], ':',
		h[a[2]>>4], h[a[2]&0xF], ':',
		h[a[1]>>4], h[a[1]&0xF], ':',
		h[a[0]>>4], h[a[0]&0xF],
	}))
}

func main() {

	println("Starting...")

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

	println("Initializing PAN211x...")

	if err := pan.Init(pan211x.BLECh37); err != nil {
		println("Init error:", err.Error())
		for {
		}
	}
	println("Init OK")

	// Read back key registers to verify chip state after Init.
	for _, r := range []struct {
		addr uint8
		name string
		want uint8
	}{
		{0x07, "WMODE_CFG0", 0xFC},
		{0x08, "WMODE_CFG1", 0xB2},
		{0x19, "PKT_EXT_CFG", 0x60},
		{0x1A, "WHITEN_CFG", 0xD3},
		{0x1B, "TXHDR0_CFG", 0x42},
		{0x39, "RF_CH", 0x02},
		{0x6F, "MISC_CFG", 0x10},
	} {
		v, err := regs.Read(r.addr)
		if err != nil {
			println(r.name, "read err:", err.Error())
		} else if v != r.want {
			println(r.name, "=", v, "want", r.want, "MISMATCH")
		} else {
			println(r.name, "=", v, "OK")
		}
	}

	printBTAddr()

	channels := [3]pan211x.BLEChannel{pan211x.BLECh37, pan211x.BLECh38, pan211x.BLECh39}
	ch := 0

	for {
		if err := pan.SetChannel(channels[ch]); err != nil {
			println("SetChannel error:", err.Error())
		} else if err := pan.Send(send); err != nil {
			println("Send error:", err.Error())
		} else {
			// Toggle green LED on successful TX.
			pinLedGreen.Set(!pinLedGreen.Get())
		}
		ch = (ch + 1) % 3

		if ch == 0 {
			var stats runtime.MemStats
			runtime.ReadMemStats(&stats)
			println("Alloc:", stats.Alloc)
			time.Sleep(500 * time.Millisecond)
		}
	}
}
