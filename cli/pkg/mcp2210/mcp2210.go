package mcp2210

import (
	"errors"
	"fmt"
)

// MCP2210 command opcodes (datasheet DS22288A §3).
const (
	cmdSetChipSettings = 0x21
	cmdSetGPIOValues   = 0x30
	cmdSetSPISettings  = 0x40
	cmdSPITransfer     = 0x42
)

// SPI transfer response status codes (response byte 1).
const (
	spiStatusOK         = 0x00
	spiStatusInProgress = 0xF8 // SPI transfer in progress; no data accepted
	spiStatusBusBusy    = 0xF7 // SPI bus owned by an external master
)

// GP0 pin designation: chip-select function (active-low, driven by the SPI
// engine around each transaction).
const gpDesignationChipSelect = 0x01

// maxTransfer is the largest SPI transaction Transfer accepts. The MCP2210
// moves up to 60 data bytes per HID report; the PAN211x register bursts are far
// smaller, so a single report per transaction is always sufficient.
const maxTransfer = 60

// ErrBusBusy is returned when the MCP2210 reports the SPI bus is owned by
// another master and the transfer cannot proceed.
var ErrBusBusy = errors.New("mcp2210: SPI bus busy")

// SPIConfig holds the static SPI master parameters programmed once at startup.
type SPIConfig struct {
	// BitRateHz is the SPI clock frequency in Hz.
	BitRateHz uint32
	// Mode is the SPI mode (0-3).
	Mode uint8
}

// DefaultSPIConfig is a conservative configuration suitable for the PAN211x:
// 1 MHz, SPI mode 0 (CPOL=0, CPHA=0).
var DefaultSPIConfig = SPIConfig{BitRateHz: 1_000_000, Mode: 0}

// Configure programs the chip's GPIO designations (GP0 as chip select) and the
// static SPI parameters. It must be called once before Transfer.
//
// gpioOutputs lists GPIO pins (1-8) to configure as outputs (driven low
// initially); all other GP pins stay inputs. GP0 is always the SPI chip select.
func (d *Device) Configure(cfg SPIConfig, gpioOutputs ...uint8) error {
	if cfg.BitRateHz == 0 {
		cfg = DefaultSPIConfig
	}
	var outMask uint16
	for _, p := range gpioOutputs {
		if p < 1 || p > 8 {
			return fmt.Errorf("mcp2210: GPIO output pin %d out of range 1-8", p)
		}
		outMask |= 1 << p
	}
	if _, err := d.command(buildChipSettings(outMask)); err != nil {
		return err
	}
	// Program SPI settings with a placeholder transaction size; Transfer
	// re-programs the size whenever it changes.
	if _, err := d.command(buildSPISettings(cfg, 1)); err != nil {
		return err
	}
	d.cfg = cfg
	d.lastTxBytes = 1
	d.gpioValues = 0
	return nil
}

// SetGPIO drives a GPIO pin (1-8) high or low. The pin must have been declared
// as an output via Configure; values for other pins are preserved.
func (d *Device) SetGPIO(pin uint8, high bool) error {
	if pin < 1 || pin > 8 {
		return fmt.Errorf("mcp2210: GPIO pin %d out of range 1-8", pin)
	}
	if high {
		d.gpioValues |= 1 << pin
	} else {
		d.gpioValues &^= 1 << pin
	}
	_, err := d.command(buildGPIOValues(d.gpioValues))
	return err
}

// Transfer runs one SPI transaction: it clocks out tx (asserting chip select
// for the whole transaction) and returns the bytes simultaneously clocked in.
// len(tx) must be between 1 and 60.
func (d *Device) Transfer(tx []byte) ([]byte, error) {
	if len(tx) == 0 || len(tx) > maxTransfer {
		return nil, fmt.Errorf("mcp2210: transfer length %d out of range 1-%d", len(tx), maxTransfer)
	}
	if len(tx) != d.lastTxBytes {
		if _, err := d.command(buildSPISettings(d.spiConfig(), len(tx))); err != nil {
			return nil, err
		}
		d.lastTxBytes = len(tx)
	}

	rx := make([]byte, 0, len(tx))
	first := true
	for len(rx) < len(tx) {
		var req [reportLen]byte
		if first {
			req = buildSPITransfer(tx)
			first = false
		} else {
			req = buildSPITransfer(nil) // drain remaining RX bytes
		}
		resp, err := d.command(req)
		if err != nil {
			return nil, err
		}
		got, err := parseSPITransfer(resp)
		if errors.Is(err, errSPIInProgress) {
			continue
		}
		if err != nil {
			return nil, err
		}
		rx = append(rx, got...)
	}
	return rx, nil
}

// spiConfig returns the SPI parameters last programmed via Configure, falling
// back to the default if Configure was never called.
func (d *Device) spiConfig() SPIConfig {
	if d.cfg.BitRateHz == 0 {
		return DefaultSPIConfig
	}
	return d.cfg
}

// --- pure request/response builders (unit-tested without hardware) ---

// buildChipSettings builds a Set-Chip-Settings (0x21) request that designates
// GP0 as the SPI chip-select pin and GP1-GP8 as GPIO. Pins whose bit is set in
// gpioOutputs are configured as outputs (initial value low); the rest are
// inputs.
func buildChipSettings(gpioOutputs uint16) [reportLen]byte {
	var req [reportLen]byte
	req[0] = cmdSetChipSettings
	req[4] = gpDesignationChipSelect // GP0 = chip select
	// req[5..12] (GP1..GP8) remain 0 = GPIO.
	// req[13..14] default GPIO output value = 0 (outputs start low).
	// req[15..16] default GPIO direction: 1 = input. Start all inputs, then
	// clear a bit for each requested output.
	dir := uint16(0x01FF) &^ gpioOutputs
	req[15] = byte(dir)
	req[16] = byte(dir >> 8)
	// req[17] other chip settings = 0 (no interrupt counting, bus release on).
	return req
}

// buildGPIOValues builds a Set-GPIO-Current-Pin-Value (0x30) request setting all
// GPIO output levels at once from the given bitmap (bit n = GPn).
func buildGPIOValues(values uint16) [reportLen]byte {
	var req [reportLen]byte
	req[0] = cmdSetGPIOValues
	req[4] = byte(values)
	req[5] = byte(values >> 8)
	return req
}

// buildSPISettings builds a Set-SPI-Transfer-Settings (0x40) request for the
// given clock/mode and number of bytes per transaction. Chip select GP0 idles
// high and is driven low (active) for the transaction.
func buildSPISettings(cfg SPIConfig, bytesPerTx int) [reportLen]byte {
	var req [reportLen]byte
	req[0] = cmdSetSPISettings
	// Bit rate, 32-bit little-endian.
	req[4] = byte(cfg.BitRateHz)
	req[5] = byte(cfg.BitRateHz >> 8)
	req[6] = byte(cfg.BitRateHz >> 16)
	req[7] = byte(cfg.BitRateHz >> 24)
	// Idle chip-select value: GP0 high.
	req[8] = 0x01
	req[9] = 0x00
	// Active chip-select value: GP0 low.
	req[10] = 0x00
	req[11] = 0x00
	// req[12..17] inter-byte/CS delays = 0.
	// Bytes per SPI transaction, 16-bit little-endian.
	req[18] = byte(bytesPerTx)
	req[19] = byte(bytesPerTx >> 8)
	// SPI mode.
	req[20] = cfg.Mode
	return req
}

// buildSPITransfer builds a Transfer-SPI-Data (0x42) request carrying data
// (up to 60 bytes). Passing nil issues a zero-length transfer used to retrieve
// received bytes from an in-progress transaction.
func buildSPITransfer(data []byte) [reportLen]byte {
	var req [reportLen]byte
	req[0] = cmdSPITransfer
	req[1] = byte(len(data))
	copy(req[4:], data)
	return req
}

// errSPIInProgress signals that the chip accepted no data because a transfer is
// still completing; the caller should retry to collect the remaining bytes.
var errSPIInProgress = errors.New("mcp2210: SPI transfer in progress")

// parseSPITransfer validates a Transfer-SPI-Data response and returns the bytes
// received in this report.
func parseSPITransfer(resp [reportLen]byte) ([]byte, error) {
	switch resp[1] {
	case spiStatusOK:
		n := int(resp[2])
		if n > maxTransfer {
			return nil, fmt.Errorf("mcp2210: response reports %d received bytes", n)
		}
		return resp[4 : 4+n], nil
	case spiStatusInProgress:
		return nil, errSPIInProgress
	case spiStatusBusBusy:
		return nil, ErrBusBusy
	default:
		return nil, fmt.Errorf("mcp2210: SPI transfer status 0x%02X", resp[1])
	}
}
