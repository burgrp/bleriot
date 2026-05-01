package i2c

import (
	"errors"
	"machine"
)

var ErrNoAck = errors.New("no ACK received")

// MasterI2C is a bitbang I2C master with external pull-ups on SCL and SDA.
// Open-drain is emulated: PinOutput+Low to drive 0, PinInputFloating to release.
// Implements pan211x.MasterI2C (Start/Stop/Write/Read).
type MasterI2C struct {
	scl machine.Pin
	sda machine.Pin
}

// NewMaster initialises both pins released (bus idle, pull-ups hold lines high).
func NewMaster(scl, sda machine.Pin) *MasterI2C {
	scl.Configure(machine.PinConfig{Mode: machine.PinInputFloating})
	sda.Configure(machine.PinConfig{Mode: machine.PinInputFloating})
	return &MasterI2C{scl: scl, sda: sda}
}

func (m *MasterI2C) sclLow() {
	m.scl.Configure(machine.PinConfig{Mode: machine.PinOutput})
	m.scl.Low()
}

// sclHigh releases SCL and spins until it reads high (supports clock stretching).
func (m *MasterI2C) sclHigh() {
	m.scl.Configure(machine.PinConfig{Mode: machine.PinInputFloating})
	for !m.scl.Get() {
	}
}

func (m *MasterI2C) sdaLow() {
	m.sda.Configure(machine.PinConfig{Mode: machine.PinOutput})
	m.sda.Low()
}

func (m *MasterI2C) sdaHigh() {
	m.sda.Configure(machine.PinConfig{Mode: machine.PinInputFloating})
}

// Start generates a START or Repeated START condition.
// Safe to call from bus-idle (both lines high) or mid-transaction (SCL low after ACK).
func (m *MasterI2C) Start() {
	m.sdaHigh()
	m.sclHigh()
	m.sdaLow() // SDA falls while SCL high → START / Repeated START
	m.sclLow()
}

// Stop generates a STOP condition (SDA rises while SCL high).
func (m *MasterI2C) Stop() {
	m.sdaLow()
	m.sclHigh()
	m.sdaHigh()
}

// Write sends b MSB-first and returns ErrNoAck if the slave does not ACK.
func (m *MasterI2C) Write(b uint8) error {
	for i := 7; i >= 0; i-- {
		if b>>uint(i)&1 != 0 {
			m.sdaHigh()
		} else {
			m.sdaLow()
		}
		m.sclHigh()
		m.sclLow()
	}
	m.sdaHigh() // release SDA for ACK
	m.sclHigh()
	ack := !m.sda.Get() // ACK = slave pulls SDA low
	m.sclLow()
	if !ack {
		return ErrNoAck
	}
	return nil
}

// Read clocks in one byte MSB-first.
// Sends NACK after the byte if last is true, ACK otherwise.
func (m *MasterI2C) Read(last bool) (uint8, error) {
	var b uint8
	m.sdaHigh() // release SDA so slave can drive it
	for i := 7; i >= 0; i-- {
		m.sclHigh()
		if m.sda.Get() {
			b |= 1 << uint(i)
		}
		m.sclLow()
	}
	if last {
		m.sdaHigh() // NACK
	} else {
		m.sdaLow() // ACK
	}
	m.sclHigh()
	m.sclLow()
	m.sdaHigh() // release SDA after ACK/NACK
	return b, nil
}
