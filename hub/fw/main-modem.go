//go:build tinygo

// Command (firmware) main-modem is the BleRiot "dumb radio modem": it bridges
// the host hub's COBS-framed link protocol (UART on PB6/PB7) to a PAN211x radio.
//
// The modem holds no secrets and no protocol intelligence. It only:
//   - announces itself to the host on boot with MsgHello,
//   - applies MsgConfigRadio (channel + receive address) to the radio,
//   - transmits MsgSend payloads over the air,
//   - forwards received radio packets to the host as MsgRecv,
//   - reports failures as MsgError.
//
// All XTEA encryption, retries, timeouts and subscription bookkeeping live on
// the host (see PROTOCOL.md). Debug logging uses println() over SEGGER RTT,
// which is independent of the USART1 host link.
package main

import (
	"machine"
	"runtime"
	"time"

	"protocol"
	"hub/link"

	"github.com/burgrp/tinygo-drivers/bb/spi"
	"github.com/burgrp/tinygo-drivers/pan211x"
)

const (
	pinLedRed   = machine.PB0
	pinLedGreen = machine.PB1

	// PAN211x over 3-wire SPI.
	pinSpiSck  = machine.PA9  // SCK  → PAN211x pin 2
	pinSpiData = machine.PA7  // DATA → PAN211x pin 3, bidirectional
	pinSpiCsn  = machine.PA10 // CSN  → PAN211x pin 1, active-low

	// Host link over USART1.
	pinUartTx = machine.PB6
	pinUartRx = machine.PB7
	uartBaud  = 115200
)

// maxFrame bounds a single COBS frame. The largest message is MsgSend
// (1 type + 4 address + 13 payload = 18 body bytes, ~20 after COBS); 64 leaves
// ample margin while keeping the decoder's static buffers tiny.
const maxFrame = 64

func main() {
	println("BleRiot modem starting...")

	pinLedGreen.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinLedRed.Configure(machine.PinConfig{Mode: machine.PinOutput})

	// Host link: USART1 on PB6 (TX) / PB7 (RX), both alternate function AF0.
	// RX is interrupt-driven into a ring buffer; we drain it from the main loop.
	pinUartTx.Configure(machine.PinConfig{Mode: machine.PinAlternate})
	pinUartTx.SetAltFunc(0)
	pinUartRx.Configure(machine.PinConfig{Mode: machine.PinAlternate})
	pinUartRx.SetAltFunc(0)
	uart := machine.DefaultUART
	must(uart.Configure(machine.UARTConfig{BaudRate: uartBaud}))

	// Radio: PAN211x in BLE LongRange mode. Init runs the full calibration
	// sequence; the host later sets the channel/address via MsgConfigRadio.
	radio := pan211x.NewDriverBLELongRange(
		pan211x.NewRegistersSPI(spi.NewMaster(pinSpiSck, pinSpiData), pinSpiCsn))
	must(radio.Init(pan211x.ConfigBLELongRange{
		PayloadLen:      protocol.PacketLen,
		SerialInterface: pan211x.SerialInterfaceSPI3W,
		SpreadFactor:    pan211x.SpreadFactorS8,
	}))
	println("radio OK")

	m := &modem{uart: uart, radio: radio, dec: link.NewDecoder(maxFrame)}
	m.txBuf = make([]byte, 0, maxFrame)

	// Announce on boot so the host can detect a firmware/protocol mismatch.
	m.sendHello()

	m.run()
}

// modem holds the firmware's long-lived state. All buffers are allocated once
// (--gc leaking, 8 KB RAM); the run loop never allocates.
type modem struct {
	uart  *machine.UART
	radio *pan211x.DriverBLELongRange
	dec   *link.Decoder

	txBuf   []byte                   // reusable link-frame encode buffer
	recvBuf [protocol.PacketLen]byte // reusable radio receive buffer
}

// run is the single cooperative loop: drain the UART into the link decoder,
// dispatch any complete host commands, then poll the radio for one received
// packet. It never returns.
func (m *modem) run() {
	for {
		// Drain everything the UART has buffered so far.
		for m.uart.Buffered() > 0 {
			b, err := m.uart.ReadByte()
			if err != nil {
				break
			}
			msg, ok, derr := m.dec.Push(b)
			if derr != nil {
				// Malformed/short or unrecognised frame; the decoder resyncs
				// on the next delimiter.
				m.sendError(link.ErrBadFrame)
				continue
			}
			if ok {
				m.dispatch(msg)
			}
		}

		// Forward at most one received packet per iteration.
		if n, ok := m.radio.Receive(m.recvBuf[:]); ok && n == protocol.PacketLen {
			m.sendRecv(m.recvBuf[:])
			pinLedGreen.Set(!pinLedGreen.Get())
		}

		runtime.Gosched()
	}
}

// dispatch handles one decoded host→modem command. msg.Payload aliases the
// decoder's internal buffer and is consumed synchronously here (the radio copies
// it into the TX FIFO), so it is never retained.
func (m *modem) dispatch(msg link.Message) {
	switch msg.Type {
	case link.MsgConfigRadio:
		if err := m.radio.SetChannel(msg.Channel); err != nil {
			println("config channel err:", err.Error())
			m.sendError(link.ErrTxFailed)
			return
		}
		if err := m.radio.EnableRxAddress(0, msg.Addr); err != nil {
			println("config address err:", err.Error())
			m.sendError(link.ErrTxFailed)
			return
		}
		// Re-announce so a host that (re)connected after our boot relearns the
		// protocol version; the UART link has no reconnect event of its own.
		m.sendHello()

	case link.MsgSend:
		if len(msg.Payload) != protocol.PacketLen {
			m.sendError(link.ErrBadFrame)
			return
		}
		if err := m.radio.Send(msg.Addr, msg.Payload); err != nil {
			m.sendError(link.ErrTxFailed)
			return
		}
		pinLedRed.Set(!pinLedRed.Get())

	default:
		// Hello/Recv/Error are modem→host only; receiving one is a host bug.
		m.sendError(link.ErrUnknownType)
	}
}

func (m *modem) writeMsg(msg link.Message) {
	m.txBuf = link.AppendMessage(m.txBuf[:0], msg)
	_, _ = m.uart.Write(m.txBuf)
}

func (m *modem) sendHello() {
	m.writeMsg(link.Message{Type: link.MsgHello, Version: link.ProtocolVersion})
}
func (m *modem) sendError(code byte) { m.writeMsg(link.Message{Type: link.MsgError, Code: code}) }
func (m *modem) sendRecv(pkt []byte) { m.writeMsg(link.Message{Type: link.MsgRecv, Payload: pkt}) }

// must halts with a visible blink pattern if a one-time setup step fails. There
// is no recovery from a radio that will not initialise.
func must(err error) {
	if err == nil {
		return
	}
	println("fatal:", err.Error())
	for {
		pinLedRed.High()
		time.Sleep(100 * time.Millisecond)
		pinLedRed.Low()
		time.Sleep(100 * time.Millisecond)
	}
}
