package pan211x

import (
	"errors"
	"runtime"
	"time"
)

// maxFIFO is the maximum FIFO size in bytes (shared between TX and RX in normal mode).
const maxFIFO = 128

// BLE advertising access address (per BLE spec, little-endian byte order).
const (
	bleAccAddr0 = 0xD6
	bleAccAddr1 = 0xBE
	bleAccAddr2 = 0x89
	bleAccAddr3 = 0x8E
)

// Register addresses (7-bit). The 8-bit access byte sent over I2C/SPI is formed by
// shifting left by 1 and ORing with the R/W bit (0=write, 1=read per timing diagrams).
const (
	regTRX_FIFO      = 0x01 // FIFO read/write access point
	regSTATE_CFG     = 0x02 // operating mode control
	regWMODE_CFG0    = 0x07 // work mode: CRC, protocol, whitening, endian
	regWMODE_CFG1    = 0x08 // work mode: FIFO size, enhance, address width
	regRXPLLEN_CFG   = 0x09 // RX payload length (fixed-length modes)
	regTXPLLEN_CFG   = 0x0A // TX payload length
	regPIPE0_RXADDR0 = 0x0F // pipe0 RX address bytes 0-3
	regPIPE0_RXADDR1 = 0x10
	regPIPE0_RXADDR2 = 0x11
	regPIPE0_RXADDR3 = 0x12
	regTXADDR0       = 0x14 // TX address bytes 0-3
	regTXADDR1       = 0x15
	regTXADDR2       = 0x16
	regTXADDR3       = 0x17
	regWHITEN_CFG    = 0x1A // whitening: WHITEN_SKIP_ADDR[7] | WHITEN_SEED[6:0]
	regRF_CHANNEL    = 0x39 // channel: frequency = 2400 + RF_CH [MHz]
	regRFIRQFLG      = 0x73 // interrupt flags (write 1 to clear)
	regSTATUS3       = 0x77 // received payload length
)

// STATE_CFG values.
// Non-deepsleep states need EN_LS_3V(bit6)|POR_RSTL(bit5)|ISO_TO_0(bit4) all set.
const (
	stateSleep = 0x01 // Sleep from Deepsleep: only mode bits, ISO_TO_0=0
	stateSTB3  = 0x74 // Standby3: 0b0111_0100
	stateTX    = 0x75 // TX mode:  0b0111_0101
	stateRX    = 0x76 // RX mode:  0b0111_0110
)

// RFIRQFLG bits (write 1 to clear).
const (
	irqTX = 1 << 7 // packet transmission complete
	irqRX = 1 << 0 // correct packet received
)

// Errors returned by Driver methods.
var (
	ErrPayloadTooLarge = errors.New("payload too large")
	ErrTimeout         = errors.New("radio timeout")
)

// Registers abstracts the physical bus (I2C or SPI) for register access.
type Registers interface {
	Read(reg uint8) (uint8, error)
	Write(reg uint8, value uint8) error
	// WriteBuffer writes len(data) bytes to reg in a single bus transaction.
	WriteBuffer(reg uint8, data []byte) error
	// ReadBuffer reads len(buf) bytes from reg into buf.
	ReadBuffer(reg uint8, buf []byte) error
}

// BLEChannel holds the register values for a BLE advertising channel.
type BLEChannel struct {
	rfCh      uint8 // RF_CH register value: frequency = 2400 + rfCh [MHz]
	whitenCfg uint8 // WHITEN_CFG: bit7=WHITEN_SKIP_ADDR=1 | bits[6:0]=BLE channel LFSR seed
}

// BLE advertising channel configurations.
// BLE whitening seed = BLE channel index | 0x40, with WHITEN_SKIP_ADDR=1 for BLE compliance.
var (
	BLECh37 = BLEChannel{rfCh: 2, whitenCfg: 0xE5}  // 2402 MHz, channel index 37
	BLECh38 = BLEChannel{rfCh: 26, whitenCfg: 0xE6} // 2426 MHz, channel index 38
	BLECh39 = BLEChannel{rfCh: 80, whitenCfg: 0xE7} // 2480 MHz, channel index 39
)

// Driver provides BLE packet send and receive over the PAN211x transceiver.
type Driver struct {
	registers Registers
}

// NewDriver creates a Driver using the given Registers implementation.
func NewDriver(registers Registers) *Driver {
	return &Driver{registers: registers}
}

// Init wakes the chip from Deepsleep and configures it for BLE advertising mode
// on the given channel. Must be called once before Send or Receive.
func (d *Driver) Init(ch BLEChannel) error {
	// From Deepsleep, only registers 0x00-0x06 are accessible.
	// Write Sleep mode (ISO_TO_0=0 for the Deepsleep→Sleep transition).
	if err := d.registers.Write(regSTATE_CFG, stateSleep); err != nil {
		return err
	}
	// Allow the 32 MHz crystal to start oscillating (typ. 75-300 µs).
	time.Sleep(400 * time.Microsecond)

	// Transition to STB3, setting EN_LS_3V, POR_RSTL, and ISO_TO_0.
	if err := d.registers.Write(regSTATE_CFG, stateSTB3); err != nil {
		return err
	}
	time.Sleep(100 * time.Microsecond)

	// WMODE_CFG0: CRC_MODE=11 (3-byte CRC), WORK_MODE=11 (BLE),
	//             WHITEN_ENABLE=1, CRC_SKIP_ADDR=0, TX_NOACK=0, ENDIAN=0 (little-endian).
	// 0b11_11_1_0_0_0 = 0xF8
	if err := d.registers.Write(regWMODE_CFG0, 0xF8); err != nil {
		return err
	}

	// WMODE_CFG1: RX_GOON=1, FIFO_128_EN=1 (required for BLE mode),
	//             ENHANCE=0 (normal), ADDR_BYTE_LENGTH=10 (4 bytes for BLE access address).
	// 0b1_0_1_0_0_0_10 = 0xA2
	if err := d.registers.Write(regWMODE_CFG1, 0xA2); err != nil {
		return err
	}

	// Set BLE advertising access address (0x8E89BED6) for pipe0 RX and TX.
	if err := d.registers.Write(regPIPE0_RXADDR0, bleAccAddr0); err != nil {
		return err
	}
	if err := d.registers.Write(regPIPE0_RXADDR1, bleAccAddr1); err != nil {
		return err
	}
	if err := d.registers.Write(regPIPE0_RXADDR2, bleAccAddr2); err != nil {
		return err
	}
	if err := d.registers.Write(regPIPE0_RXADDR3, bleAccAddr3); err != nil {
		return err
	}
	if err := d.registers.Write(regTXADDR0, bleAccAddr0); err != nil {
		return err
	}
	if err := d.registers.Write(regTXADDR1, bleAccAddr1); err != nil {
		return err
	}
	if err := d.registers.Write(regTXADDR2, bleAccAddr2); err != nil {
		return err
	}
	if err := d.registers.Write(regTXADDR3, bleAccAddr3); err != nil {
		return err
	}

	// Set RF channel (frequency = 2400 + rfCh MHz).
	if err := d.registers.Write(regRF_CHANNEL, ch.rfCh); err != nil {
		return err
	}

	// Set whitening: WHITEN_SKIP_ADDR=1 (BLE-conformant, whitening covers Header+Payload only)
	// with the channel-specific LFSR seed.
	if err := d.registers.Write(regWHITEN_CFG, ch.whitenCfg); err != nil {
		return err
	}

	return nil
}

// Send transmits a BLE packet. payload must be the complete PDU content to be placed
// in the TX FIFO. For BLE advertising this is: [Header(1)][Length(1)][AdvA(6)][AdvData(N)].
// Maximum supported payload is 128 bytes; BLE advertising PDUs are typically ≤39 bytes.
func (d *Driver) Send(payload []byte) error {
	if len(payload) > maxFIFO {
		return ErrPayloadTooLarge
	}

	// Ensure STB3 before touching FIFO (FIFO access requires STB3 or higher).
	if err := d.registers.Write(regSTATE_CFG, stateSTB3); err != nil {
		return err
	}

	// Write payload into the TX FIFO.
	if err := d.registers.WriteBuffer(regTRX_FIFO, payload); err != nil {
		return err
	}

	// Set the TX payload length register.
	if err := d.registers.Write(regTXPLLEN_CFG, uint8(len(payload))); err != nil {
		return err
	}

	// Clear any stale TX interrupt flag (write 1 to clear).
	if err := d.registers.Write(regRFIRQFLG, irqTX); err != nil {
		return err
	}

	// Switch to TX mode. In single-packet mode (default) the chip returns to STB3
	// automatically after transmitting one packet.
	if err := d.registers.Write(regSTATE_CFG, stateTX); err != nil {
		return err
	}

	// Poll RFIRQFLG for the TX-complete interrupt.
	// TX settling ~73 µs + time-on-air + ~26 µs exit; allow generous timeout.
	for i := 0; i < 5000; i++ {
		flags, err := d.registers.Read(regRFIRQFLG)
		if err != nil {
			// Best-effort return to STB3; primary error takes precedence.
			_ = d.registers.Write(regSTATE_CFG, stateSTB3)
			return err
		}
		if flags&irqTX != 0 {
			// TX complete. Clear the flag; chip is already back in STB3.
			return d.registers.Write(regRFIRQFLG, irqTX)
		}
		runtime.Gosched()
	}

	// Timeout: best-effort return to STB3; primary error takes precedence.
	_ = d.registers.Write(regSTATE_CFG, stateSTB3)
	return ErrTimeout
}

// Receive switches the chip to RX mode and waits for one BLE packet to arrive.
// The received PDU bytes are written into buf; the actual number of bytes received
// is returned. In BLE mode the chip auto-detects packet length from the PDU LENGTH field.
// len(buf) must be large enough to hold the expected PDU (≥39 bytes for full BLE ADV).
func (d *Driver) Receive(buf []byte) (int, error) {
	// Clear any stale RX interrupt flag.
	if err := d.registers.Write(regRFIRQFLG, irqRX); err != nil {
		return 0, err
	}

	// Switch to RX mode (single-packet; chip returns to STB3 after one packet).
	if err := d.registers.Write(regSTATE_CFG, stateRX); err != nil {
		return 0, err
	}

	// Poll RFIRQFLG for the RX interrupt.
	for i := 0; i < 500000; i++ {
		flags, err := d.registers.Read(regRFIRQFLG)
		if err != nil {
			// Best-effort return to STB3; primary error takes precedence.
			_ = d.registers.Write(regSTATE_CFG, stateSTB3)
			return 0, err
		}
		if flags&irqRX != 0 {
			// Read the actual received payload length from STATUS3.
			rxLen, err := d.registers.Read(regSTATUS3)
			if err != nil {
				// Best-effort return to STB3; primary error takes precedence.
				_ = d.registers.Write(regSTATE_CFG, stateSTB3)
				return 0, err
			}
			n := int(rxLen)
			if n > len(buf) {
				n = len(buf)
			}

			// Read payload bytes from the RX FIFO.
			if err := d.registers.ReadBuffer(regTRX_FIFO, buf[:n]); err != nil {
				// Best-effort return to STB3; primary error takes precedence.
				_ = d.registers.Write(regSTATE_CFG, stateSTB3)
				return 0, err
			}

			// RX complete. Clear interrupt flag and return to STB3.
			if err := d.registers.Write(regRFIRQFLG, irqRX); err != nil {
				return 0, err
			}
			return n, d.registers.Write(regSTATE_CFG, stateSTB3)
		}
		runtime.Gosched()
	}

	// Timeout: best-effort return to STB3; primary error takes precedence.
	_ = d.registers.Write(regSTATE_CFG, stateSTB3)
	return 0, ErrTimeout
}
