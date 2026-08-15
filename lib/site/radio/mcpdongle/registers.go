package mcpdongle

// transport is the SPI transaction surface the register layer needs. It is
// satisfied by *mcp2210.Device and faked in tests.
type transport interface {
	// Transfer clocks out tx and returns the bytes simultaneously clocked in
	// (one received byte per transmitted byte), all under a single chip-select
	// assertion.
	Transfer(tx []byte) ([]byte, error)
}

// registers implements pan211x.Registers over an MCP2210 SPI transport.
//
// The PAN211x 3-wire register protocol uses an access byte of reg<<1 for reads
// and reg<<1|1 for writes, followed by the data byte(s); each operation is a
// single chip-select assertion, i.e. one Transfer.
type registers struct {
	t   transport
	err error
}

func newRegisters(t transport) *registers {
	return &registers{t: t}
}

// takeError returns and clears the first transport error since the previous
// call. The PAN211x Receive API reports register failures as "no packet", so
// the host adapter preserves them here for the dongle supervisor.
func (r *registers) takeError() error {
	err := r.err
	r.err = nil
	return err
}

func (r *registers) recordError(err error) {
	if err != nil && r.err == nil {
		r.err = err
	}
}

// Read returns the byte at reg. The access byte is clocked out first; the
// register value is clocked in during the trailing dummy byte.
func (r *registers) Read(reg uint8) (uint8, error) {
	rx, err := r.t.Transfer([]byte{reg << 1, 0x00})
	if err != nil {
		r.recordError(err)
		return 0, err
	}
	return rx[len(rx)-1], nil
}

// Write stores value at reg.
func (r *registers) Write(reg uint8, value uint8) error {
	_, err := r.t.Transfer([]byte{reg<<1 | 1, value})
	r.recordError(err)
	return err
}

// WriteBuffer writes data starting at reg in a single transaction.
func (r *registers) WriteBuffer(reg uint8, data []byte) error {
	tx := make([]byte, 0, len(data)+1)
	tx = append(tx, reg<<1|1)
	tx = append(tx, data...)
	_, err := r.t.Transfer(tx)
	r.recordError(err)
	return err
}

// ReadBuffer reads len(buf) bytes starting at reg in a single transaction.
func (r *registers) ReadBuffer(reg uint8, buf []byte) error {
	tx := make([]byte, len(buf)+1)
	tx[0] = reg << 1
	rx, err := r.t.Transfer(tx)
	if err != nil {
		r.recordError(err)
		return err
	}
	copy(buf, rx[1:])
	return nil
}
