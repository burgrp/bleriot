package spi

import "machine"

// Master is a bit-bang 3-wire SPI master (CPOL=0, CPHA=0, MSB first).
// DATA is a single bidirectional pin shared for both TX and RX, matching the
// PAN211x 3-wire SPI interface. The pin direction is managed per the timing
// requirement: DATA must switch to input before the 8th SCK falling edge of
// a read command so the chip can drive the response without bus contention.
type Master struct {
	sck  machine.Pin
	data machine.Pin
}

// NewMaster creates a bit-bang SPI master and configures SCK as output-low.
func NewMaster(sck, data machine.Pin) *Master {
	sck.Configure(machine.PinConfig{Mode: machine.PinOutput})
	sck.Low()
	return &Master{sck: sck, data: data}
}

// WriteByte sends b MSB-first with DATA as output.
// On the last bit, DATA is switched to input before the 8th falling edge so
// the PAN211x can safely take over the line for a subsequent ReadByte.
func (m *Master) WriteByte(b byte) error {
	m.data.Configure(machine.PinConfig{Mode: machine.PinOutput})
	for i := 7; i >= 1; i-- {
		if b&(1<<uint(i)) != 0 {
			m.data.High()
		} else {
			m.data.Low()
		}
		m.sck.High()
		m.sck.Low()
	}
	// Last bit: set DATA, clock high, then switch DATA to input BEFORE the
	// 8th falling edge so the chip can drive DATA for a following read.
	if b&1 != 0 {
		m.data.High()
	} else {
		m.data.Low()
	}
	m.sck.High()
	m.data.Configure(machine.PinConfig{Mode: machine.PinInput})
	m.sck.Low() // 8th falling edge — chip may now drive DATA
	return nil
}

// ReadByte clocks in 8 bits MSB-first with DATA as input (chip drives the line).
func (m *Master) ReadByte() (byte, error) {
	m.data.Configure(machine.PinConfig{Mode: machine.PinInput})
	var b byte
	for i := 7; i >= 0; i-- {
		m.sck.High()
		if m.data.Get() {
			b |= 1 << uint(i)
		}
		m.sck.Low()
	}
	return b, nil
}
