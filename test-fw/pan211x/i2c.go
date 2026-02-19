package pan211x

// MasterI2C is the interface required by RegistersI2C.
type MasterI2C interface {
	WaitForBus()
	Stop()
	Write(address uint8, chunks ...[]uint8) error
	Read(address uint8, data []uint8) (int, error)
}

// RegistersI2C implements the Registers interface over I2C.
// The PAN211x I2C protocol uses an 8-bit register access byte: bits[7:1] = register
// address, bit[0] = R/W (0 = write, 1 = read, per section 10.3 timing diagrams).
type RegistersI2C struct {
	i2c MasterI2C
}

// NewRegistersI2C creates a RegistersI2C backed by the given I2C master.
func NewRegistersI2C(i2c MasterI2C) *RegistersI2C {
	return &RegistersI2C{i2c: i2c}
}

// accessWrite forms the 8-bit register access byte for a write operation.
func accessWrite(reg uint8) uint8 { return reg << 1 }

// accessRead forms the 8-bit register access byte for a read operation.
func accessRead(reg uint8) uint8 { return reg<<1 | 1 }

// Read reads one byte from the given register.
func (r *RegistersI2C) Read(reg uint8) (uint8, error) {
	r.i2c.WaitForBus()
	defer r.i2c.Stop()

	if err := r.i2c.Write(PAN211xAddress, []uint8{accessRead(reg)}); err != nil {
		return 0, err
	}

	data := make([]uint8, 1)
	if _, err := r.i2c.Read(PAN211xAddress, data); err != nil {
		return 0, err
	}

	return data[0], nil
}

// Write writes one byte to the given register.
func (r *RegistersI2C) Write(reg uint8, value uint8) error {
	r.i2c.WaitForBus()
	defer r.i2c.Stop()

	return r.i2c.Write(PAN211xAddress, []uint8{accessWrite(reg), value})
}

// WriteBuffer writes data to the given register.
func (r *RegistersI2C) WriteBuffer(reg uint8, data []byte) error {
	r.i2c.WaitForBus()
	defer r.i2c.Stop()

	return r.i2c.Write(PAN211xAddress, []uint8{accessWrite(reg)}, data)
}

// ReadBuffer reads len(buf) bytes from the given register into buf.
func (r *RegistersI2C) ReadBuffer(reg uint8, buf []byte) error {
	r.i2c.WaitForBus()
	defer r.i2c.Stop()

	if err := r.i2c.Write(PAN211xAddress, []uint8{accessRead(reg)}); err != nil {
		return err
	}

	_, err := r.i2c.Read(PAN211xAddress, buf)
	return err
}
