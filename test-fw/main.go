package main

import (
	"device/py32"
	"machine"
	"test-fw/bleriot"
	"test-fw/i2c"
	"test-fw/pan211x"
	"time"
	"unsafe"
)

const (
	pinUartTx   = machine.PB6
	pinUartRx   = machine.PB7
	pinLedRed   = machine.PB0
	pinLedGreen = machine.PB1
	pinI2cSda   = machine.PA7
	pinI2cSdaAf = 12
	pinI2cScl   = machine.PA9
	pinI2cSclAf = 6
	pinRoleHub  = machine.PA0 // tie to GND = hub, leave floating = node
)

// uidBase is the PY32F030 Product Unique Identifier base address (16 bytes).
// See PY32F030 Reference Manual §4.3.
const uidBase = 0x1FFF_0E00

// deviceAddr computes a BleRiot device address from the MCU's 96-bit UID
// using IEEE CRC32. No heap allocation — operates on a fixed memory window.
func deviceAddr() bleriot.Address {
	uid := (*[16]byte)(unsafe.Pointer(uintptr(uidBase)))
	return bleriot.Address(crc32(uid[:]))
}

// crc32 computes IEEE CRC32 of data without using the hash/crc32 package.
// All operations on the stack; no allocation.
func crc32(data []byte) uint32 {
	crc := uint32(0xFFFFFFFF)
	for _, b := range data {
		crc ^= uint32(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xEDB88320
			} else {
				crc >>= 1
			}
		}
	}
	return ^crc
}

// printAddr prints a device address as hex to the RTT console.
func printAddr(label string, addr bleriot.Address) {
	const h = "0123456789ABCDEF"
	v := uint32(addr)
	println(label, string([]byte{
		'0', 'x',
		h[(v>>28)&0xF], h[(v>>24)&0xF],
		h[(v>>20)&0xF], h[(v>>16)&0xF],
		h[(v>>12)&0xF], h[(v>>8)&0xF],
		h[(v>>4)&0xF], h[v&0xF],
	}))
}

// i2cBusRecovery clocks out a stuck I2C slave by toggling SCL up to 9 times.
// A mid-transaction MCU reset can leave the slave holding SDA low waiting for
// more clock edges; without this the BUSY flag never clears.
func i2cBusRecovery() {
	pinI2cScl.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinI2cSda.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinI2cScl.High()
	pinI2cSda.High()
	time.Sleep(10 * time.Microsecond)

	for i := 0; i < 9; i++ {
		if pinI2cSda.Get() {
			break
		}
		pinI2cScl.Low()
		time.Sleep(5 * time.Microsecond)
		pinI2cScl.High()
		time.Sleep(5 * time.Microsecond)
	}

	// STOP condition: SDA low → high while SCL is high.
	pinI2cSda.Low()
	time.Sleep(5 * time.Microsecond)
	pinI2cSda.High()
	time.Sleep(5 * time.Microsecond)
}

func initHardware(ownAddr bleriot.Address) *pan211x.Driver {
	machine.ConfigureUARTPin(pinUartTx, 0)
	machine.ConfigureUARTPin(pinUartRx, 0)

	pinLedGreen.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pinLedRed.Configure(machine.PinConfig{Mode: machine.PinOutput})

	// Role select pin: input with pull-up. GND = hub, floating = node.
	pinRoleHub.Configure(machine.PinConfig{Mode: machine.PinInputPullup})

	i2cBusRecovery()
	pinI2cSda.Configure(machine.PinConfig{Mode: machine.PinAlternate})
	pinI2cSda.SetAltFunc(pinI2cSdaAf)
	pinI2cScl.Configure(machine.PinConfig{Mode: machine.PinAlternate})
	pinI2cScl.SetAltFunc(pinI2cSclAf)
	py32.RCC.APBENR1.SetBits(py32.RCC_APBENR1_I2CEN)
	py32.RCC.APBRSTR1.SetBits(py32.RCC_APBRSTR1_I2CRST)
	py32.RCC.APBRSTR1.ClearBits(py32.RCC_APBRSTR1_I2CRST)

	i2cMaster := i2c.NewMaster(py32.I2C, 24_000_000, 100_000)
	regs := pan211x.NewRegistersI2C(i2cMaster)

	// BleRiot RF config: own device address as hardware RX filter, 2440 MHz, 250 kbps.
	// Whitening seed for RF_CH=40: index=40, seed=40|0x40=0x68, bit-reversed=0x16,
	// WHITEN_CFG = 0x80|0x16 = 0x96.
	cfg := pan211x.Config{
		OwnAddr:   pan211x.Address(ownAddr),
		RFChannel: 40, // 2440 MHz
		WhitenCfg: 0x96,
		DataRate:  pan211x.DataRate250kbps,
	}
	return pan211x.NewDriver(regs, cfg)
}

func main() {
	println("BleRiot starting...")

	addr := deviceAddr()
	printAddr("Device address:", addr)

	pan := initHardware(addr)

	if err := pan.Init(); err != nil {
		println("Radio init error:", err.Error())
		for {
		}
	}
	println("Radio OK")

	stack := bleriot.NewStack(pan, addr)

	// Role is determined once at boot by the level on pinRoleHub.
	// Low (tied to GND) = hub, high (floating with pull-up) = node.
	if !pinRoleHub.Get() {
		println("Role: HUB")
		runHub(&stack)
	} else {
		println("Role: NODE")
		runNode(&stack)
	}
}

// ── Hub ──────────────────────────────────────────────────────────────────────

// hubNodeAddr is the address of the node to poll. In a real deployment this
// would be discovered; here it is pasted from the node's boot log.
// Replace with the address printed by the node on startup.
const hubNodeAddr bleriot.Address = 0xDF77AB7F // placeholder — update from node boot log

func runHub(s *bleriot.Stack) {
	var seq uint8

	// nextSeq returns the next sequence number, skipping the reserved SeqPush (0xFF).
	nextSeq := func() uint8 {
		seq++
		if seq == bleriot.SeqPush {
			seq = 0
		}
		return seq
	}

	for {
		// ── Read register 2 (counter, R/O int32) ──────────────────────────
		curSeq := nextSeq()
		if err := s.SendReadRequest(hubNodeAddr, curSeq, 2, false); err != nil {
			println("send err:", err.Error())
		} else {
			resp, ok := waitResponse(s, hubNodeAddr, curSeq, 50*time.Millisecond)
			if ok {
				println("REG2 (counter):", resp.VALUE)
			} else {
				println("REG2 timeout")
			}
		}

		// ── Read register 1 (float-backed int32) ──────────────────────────
		curSeq = nextSeq()
		if err := s.SendReadRequest(hubNodeAddr, curSeq, 1, false); err != nil {
			println("send err:", err.Error())
		} else {
			resp, ok := waitResponse(s, hubNodeAddr, curSeq, 50*time.Millisecond)
			if ok {
				println("REG1:", resp.VALUE)
			} else {
				println("REG1 timeout")
			}
		}

		// ── Write register 0 (LED) — toggle each cycle ────────────────────
		curSeq = nextSeq()
		ledVal := int32(0)
		if seq%4 < 2 {
			ledVal = 1
		}
		if err := s.SendWriteRequest(hubNodeAddr, curSeq, 0, ledVal); err != nil {
			println("send err:", err.Error())
		} else {
			resp, ok := waitResponse(s, hubNodeAddr, curSeq, 50*time.Millisecond)
			if ok {
				println("REG0 (LED) actual:", resp.VALUE)
			} else {
				println("REG0 timeout")
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}

// waitResponse polls for a response matching (src==from, seq==wantSeq) for up
// to timeout. Returns the packet and true on match, zero-value and false on timeout.
// Non-blocking poll loop — no goroutines, no channels.
func waitResponse(s *bleriot.Stack, from bleriot.Address, wantSeq uint8, timeout time.Duration) (bleriot.Packet, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		p, ok := s.Receive()
		if ok && p.SRC == from && p.SEQ == wantSeq && p.IsResponse() {
			return p, true
		}
	}
	return bleriot.Packet{}, false
}

// ── Node ─────────────────────────────────────────────────────────────────────

// Node register IDs.
const (
	regLED     = 0 // int32 R/W: LED on/off (0 or 1)
	regBacked  = 1 // int32 R/W: arbitrary value backed by variable
	regCounter = 2 // int32 R/O: increments every second
)

// nodeState holds the node's register values. On the stack in runNode.
type nodeState struct {
	led     int32
	backed  int32
	counter int32
}

func runNode(s *bleriot.Stack) {
	var st nodeState
	lastTick := time.Now()

	for {
		// Increment counter every second.
		if time.Since(lastTick) >= time.Second {
			st.counter++
			lastTick = time.Now()
		}

		p, ok := s.Receive()
		if !ok {
			continue
		}

		if !p.IsRequest() {
			continue // ignore stray responses
		}

		switch p.REG {
		case regLED:
			if p.IsWrite() {
				if p.VALUE != 0 {
					st.led = 1
					pinLedRed.High()
				} else {
					st.led = 0
					pinLedRed.Low()
				}
				_ = s.SendWriteResponse(p.SRC, p.SEQ, regLED, st.led)
			} else {
				_ = s.SendReadResponse(p.SRC, p.SEQ, regLED, st.led)
			}

		case regBacked:
			if p.IsWrite() {
				st.backed = p.VALUE
				_ = s.SendWriteResponse(p.SRC, p.SEQ, regBacked, st.backed)
			} else {
				_ = s.SendReadResponse(p.SRC, p.SEQ, regBacked, st.backed)
			}

		case regCounter:
			// Read-only: ignore write value, always echo current counter.
			_ = s.SendReadResponse(p.SRC, p.SEQ, regCounter, st.counter)

		default:
			// Unknown register: respond with READ_RESP, value 0.
			_ = s.SendReadResponse(p.SRC, p.SEQ, p.REG, 0)
		}

		// Toggle green LED on each handled packet.
		pinLedGreen.Set(!pinLedGreen.Get())
	}
}
