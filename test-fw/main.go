package main

import (
	"device/py32"
	"errors"
	"machine"
	"runtime"
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

	i2c := NewPyI2CMaster(py32.I2C)

	for {

		err := i2c.write(0x71, []uint8{0x55, 0xF0}, true)
		if err != nil {
			println("Write error: ", err.Error())
		}

		time.Sleep(500 * time.Millisecond)
	}

	// println("This is the end...")
	// for {
	// }
}

type PyI2CMaster struct {
	peri *py32.I2C_Type
}

var ErrNoAck = errors.New("no ACK received")

func NewPyI2CMaster(peri *py32.I2C_Type) *PyI2CMaster {

	peri.SetCCR_F_S(1000)
	peri.SetTRISE(10)
	py32.I2C.SetCR1_PE(1)

	return &PyI2CMaster{peri: peri}
}

func (i2c *PyI2CMaster) writeByte(b uint8) error {
	i2c.peri.DR.Set(uint32(b))

	for {
		sr1 := py32.I2C.SR1.Get()
		_ = py32.I2C.SR2.Get()

		if sr1&py32.I2C_SR1_AF != 0 {
			return ErrNoAck
		}

		if sr1&py32.I2C_SR1_TXE != 0 {
			break
		}

		runtime.Gosched()
	}

	return nil
}

func (i2c *PyI2CMaster) write(address uint8, data []uint8, stop bool) error {

	py32.I2C.CR1.SetBits(py32.I2C_CR1_START)
	for !py32.I2C.SR1.HasBits(py32.I2C_SR1_SB) {
		runtime.Gosched()
	}

	if stop {
		defer py32.I2C.CR1.SetBits(py32.I2C_CR1_STOP)
	}

	err := i2c.writeByte(address << 1)
	if err != nil {
		return err
	}

	for _, b := range data {

		err := i2c.writeByte(b)
		if err != nil {
			return err
		}

	}

	return nil
}

func hex32(v uint32) string {
	digits := "0123456789ABCDEF"
	return "0x" + string([]byte{
		digits[v>>28&0xf],
		digits[v>>24&0xf],
		digits[v>>20&0xf],
		digits[v>>16&0xf],
		digits[v>>12&0xf],
		digits[v>>8&0xf],
		digits[v>>4&0xf],
		digits[v&0xf],
	})
}
