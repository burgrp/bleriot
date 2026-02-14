package i2c

import (
	"device/py32"
	"errors"
)

var (
	ErrPEC     = errors.New("PEC error")
	ErrOverrun = errors.New("overrun/underrun")
	ErrNoAck   = errors.New("no ACK received")
	ErrArlo    = errors.New("arbitration lost")
	ErrBERR    = errors.New("bus error")
)

func clearErrors(peri *py32.I2C_Type) {
	peri.SR1.ClearBits(py32.I2C_SR1_PECERR | py32.I2C_SR1_OVR | py32.I2C_SR1_AF | py32.I2C_SR1_ARLO | py32.I2C_SR1_BERR)
}

func checkErrors(peri *py32.I2C_Type) error {
	sr1 := peri.SR1.Get()
	clearErrors(peri)

	if sr1&py32.I2C_SR1_PECERR != 0 {
		return ErrPEC
	}

	if sr1&py32.I2C_SR1_OVR != 0 {
		return ErrOverrun
	}

	if sr1&py32.I2C_SR1_AF != 0 {
		return ErrNoAck
	}

	if sr1&py32.I2C_SR1_ARLO != 0 {
		return ErrArlo
	}

	if sr1&py32.I2C_SR1_BERR != 0 {
		return ErrBERR
	}

	return nil
}
