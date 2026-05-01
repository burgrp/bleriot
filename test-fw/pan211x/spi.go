package pan211x

import "machine"

// SPIBus is the 3-wire (half-duplex) SPI interface required by RegistersSPI.
// Writes and reads are separated because the DATA pin is bidirectional:
// the PAN211x takes over DATA on the 8th SCK falling edge of a read command.
type SPIBus interface {
	// WriteByte sends one byte with DATA as output.
	// After the last bit, DATA must be released (set to input) before the 8th
	// falling edge so the chip can drive it for the subsequent read.
	WriteByte(b byte) error
	// ReadByte clocks in one byte with DATA as input (chip drives the line).
	ReadByte() (byte, error)
}

// RegistersSPI implements the Registers interface over 3-wire SPI.
// Protocol per SDK pan211.c: access byte = reg<<1 (read) or reg<<1|1 (write),
// followed by data bytes. All bytes for one operation share a single CS assertion.
type RegistersSPI struct {
	spi SPIBus
	cs  machine.Pin
}

// NewRegistersSPI creates a RegistersSPI backed by the given SPI bus and chip select pin.
func NewRegistersSPI(spi SPIBus, cs machine.Pin) *RegistersSPI {
	cs.High()
	cs.Configure(machine.PinConfig{Mode: machine.PinOutput})
	return &RegistersSPI{spi: spi, cs: cs}
}

// Read reads one byte from the given register.
func (r *RegistersSPI) Read(reg uint8) (uint8, error) {
	r.cs.Low()
	if err := r.spi.WriteByte(reg << 1); err != nil {
		r.cs.High()
		return 0, err
	}
	val, err := r.spi.ReadByte()
	r.cs.High()
	return val, err
}

// Write writes one byte to the given register.
func (r *RegistersSPI) Write(reg uint8, value uint8) error {
	r.cs.Low()
	if err := r.spi.WriteByte(reg<<1 | 1); err != nil {
		r.cs.High()
		return err
	}
	err := r.spi.WriteByte(value)
	r.cs.High()
	return err
}

// WriteBuffer writes data to the given register in a single CS assertion.
func (r *RegistersSPI) WriteBuffer(reg uint8, data []byte) error {
	r.cs.Low()
	if err := r.spi.WriteByte(reg<<1 | 1); err != nil {
		r.cs.High()
		return err
	}
	for _, b := range data {
		if err := r.spi.WriteByte(b); err != nil {
			r.cs.High()
			return err
		}
	}
	r.cs.High()
	return nil
}

// ReadBuffer reads len(buf) bytes from reg in a single CS assertion.
// The PAN211x FIFO advances on each clocked byte within the burst.
func (r *RegistersSPI) ReadBuffer(reg uint8, buf []byte) error {
	r.cs.Low()
	if err := r.spi.WriteByte(reg << 1); err != nil {
		r.cs.High()
		return err
	}
	for i := range buf {
		b, err := r.spi.ReadByte()
		if err != nil {
			r.cs.High()
			return err
		}
		buf[i] = b
	}
	r.cs.High()
	return nil
}
