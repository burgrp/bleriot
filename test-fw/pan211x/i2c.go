package pan211x

// PAN211xAddress is the 7-bit I2C address of the PAN211x chip.
const PAN211xAddressWrite = 0x71 << 1
const PAN211xAddressRead = 0x71<<1 | 1

// MasterI2C is the interface required by RegistersI2C.
type MasterI2C interface {
	Start()
	Stop()
	Read(last bool) (uint8, error)
	Write(b uint8) error
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
	r.i2c.Start()
	defer r.i2c.Stop()

	if err := r.i2c.Write(PAN211xAddressWrite); err != nil {
		return 0, err
	}

	if err := r.i2c.Write(accessRead(reg)); err != nil {
		return 0, err
	}

	r.i2c.Start()

	if err := r.i2c.Write(PAN211xAddressRead); err != nil {
		return 0, err
	}

	return r.i2c.Read(true)
}

// Write writes one byte to the given register.
func (r *RegistersI2C) Write(reg uint8, value uint8) error {
	r.i2c.Start()
	defer r.i2c.Stop()

	if err := r.i2c.Write(PAN211xAddressWrite); err != nil {
		return err
	}

	if err := r.i2c.Write(accessWrite(reg)); err != nil {
		return err
	}

	if err := r.i2c.Write(value); err != nil {
		return err
	}

	return nil
}

// WriteBuffer writes data to the given register.
func (r *RegistersI2C) WriteBuffer(reg uint8, data []byte) error {

	r.i2c.Start()
	defer r.i2c.Stop()

	if err := r.i2c.Write(PAN211xAddressWrite); err != nil {
		return err
	}

	if err := r.i2c.Write(accessWrite(reg)); err != nil {
		return err
	}

	for _, b := range data {
		if err := r.i2c.Write(b); err != nil {
			return err
		}
	}

	return nil
}

// ReadBuffer reads len(buf) bytes from reg.
func (r *RegistersI2C) ReadBuffer(reg uint8, buf []byte) error {
	r.i2c.Start()
	defer r.i2c.Stop()

	if err := r.i2c.Write(PAN211xAddressWrite); err != nil {
		return err
	}

	if err := r.i2c.Write(accessRead(reg)); err != nil {
		return err
	}

	r.i2c.Start()

	if err := r.i2c.Write(PAN211xAddressRead); err != nil {
		return err
	}

	li := len(buf) - 1
	for i := range buf {
		b, err := r.i2c.Read(i == li)
		if err != nil {
			return err
		}
		buf[i] = b
	}

	return nil
}
