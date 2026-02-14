package i2c

import (
	"device/py32"
	"runtime"
)

type Master struct {
	peri *py32.I2C_Type
}

func NewMaster(peri *py32.I2C_Type, apbClockHz int, i2cClockHz int) *Master {

	peri.SetCR2_FREQ(uint32(apbClockHz / 1e6))
	peri.SetCCR(uint32(apbClockHz / (i2cClockHz * 2)))

	apbClockkTns := 1e9 / apbClockHz
	tRiseMaxNs := 1000 // I2C protocol specification
	trise := uint32((tRiseMaxNs / apbClockkTns) + 1)
	peri.SetTRISE(trise)

	peri.SetCR1_PE(1)

	return &Master{peri: peri}
}

func (i2c *Master) writeByte(b uint8) error {
	i2c.peri.DR.Set(uint32(b))

	for {
		sr1 := i2c.peri.SR1.Get()
		_ = i2c.peri.SR2.Get()

		err := checkErrors(i2c.peri)
		if err != nil {
			return err
		}

		if sr1&py32.I2C_SR1_TXE != 0 {
			break
		}

		if sr1&py32.I2C_SR1_RXNE != 0 {
			break
		}

		runtime.Gosched()
	}

	return nil
}

func (i2c *Master) WaitForBus() {
	for i2c.peri.SR2.HasBits(py32.I2C_SR2_BUSY) {
		runtime.Gosched()
	}
}

func (i2c *Master) Write(address uint8, data []uint8) error {

	i2c.peri.CR1.SetBits(py32.I2C_CR1_START)
	for !i2c.peri.SR1.HasBits(py32.I2C_SR1_SB) {
		runtime.Gosched()
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

func (i2c *Master) Read(address uint8, data *[]uint8) (int, error) {

	i2c.peri.CR1.SetBits(py32.I2C_CR1_START)
	for !i2c.peri.SR1.HasBits(py32.I2C_SR1_SB) {
		runtime.Gosched()
	}

	err := i2c.writeByte(address<<1 | 1)
	if err != nil {
		return 0, err
	}

	read := 0
	for i := range *data {
		sr1 := i2c.peri.SR1.Get()
		_ = i2c.peri.SR2.Get()

		if sr1&py32.I2C_SR1_RXNE != 0 {
			(*data)[i] = uint8(i2c.peri.DR.Get())
			read++
		}

		err := checkErrors(i2c.peri)
		if err != nil {
			return read, err
		}

		runtime.Gosched()
	}

	return read, nil
}

func (i2c *Master) Stop() {
	i2c.peri.CR1.SetBits(py32.I2C_CR1_STOP)
	clearErrors(i2c.peri)
}
