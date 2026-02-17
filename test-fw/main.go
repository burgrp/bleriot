package main

import (
	"device/py32"
	"machine"
	"runtime"
	"test-fw/i2c"
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

	pan := NewPAN211x(i2cMaster)

	// data := make([]uint8, 2)

	for {

		// for r := 0; r <= 0x7D; r++ {

		// 	i2cMaster.WaitForBus()

		// 	err := i2cMaster.Write(0x71, []uint8{uint8(r)})
		// 	if err != nil {
		// 		println("Write error: ", err.Error())
		// 	}

		// 	read, err := i2cMaster.Read(0x71, &data)
		// 	if err != nil {
		// 		println("Read error: ", err.Error())
		// 	} else {
		// 		println("Read: ", r, read, "bytes:", data[0], data[1])
		// 	}

		// 	i2cMaster.Stop()
		// }

		println(pan.ReadRegister(0x0F))

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

const PAN211xAddress = 0x71

type MasterI2C interface {
	WaitForBus()
	Write(address uint8, data []uint8) error
	Read(address uint8, data *[]uint8) (int, error)
	Stop()
}

type PAN211x struct {
	i2c MasterI2C
}

func NewPAN211x(i2c MasterI2C) *PAN211x {
	pan := &PAN211x{i2c: i2c}
	return pan
}

func (pan *PAN211x) ReadRegister(addr uint8) (uint8, error) {

	pan.i2c.WaitForBus()
	defer pan.i2c.Stop()

	err := pan.i2c.Write(PAN211xAddress, []uint8{addr})
	if err != nil {
		return 0, err
	}

	data := make([]uint8, 1)
	_, err = pan.i2c.Read(PAN211xAddress, &data)
	if err != nil {
		return 0, err
	}

	return data[0], nil
}

func (pan *PAN211x) WriteRegister(addr uint8, value uint8) error {

	pan.i2c.WaitForBus()
	defer pan.i2c.Stop()

	err := pan.i2c.Write(PAN211xAddress, []uint8{addr, value})
	if err != nil {
		return err
	}

	return nil
}
