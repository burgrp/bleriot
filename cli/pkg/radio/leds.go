package radio

import (
	"sync"
	"time"
)

// MCP2210 GPIO pins wired to the dongle's status LEDs (see usb/usb.kicad_sch).
// Both LEDs are active-high: GPn → 330R → LED anode → cathode → GND.
const (
	ledRedPin   uint8 = 1 // GP1: lit while transmitting
	ledGreenPin uint8 = 2 // GP2: lit when a packet arrives
)

// ledHold is how long an LED stays lit after its event. A new event of the same
// kind re-arms the timer, so sustained activity keeps the LED on; it turns off
// once ledHold elapses without a repeat.
const ledHold = 100 * time.Millisecond

// led is one status LED driven by a single MCP2210 GPIO output, with an
// auto-off timer.
type led struct {
	r   *Radio
	pin uint8

	mu  sync.Mutex
	off *time.Timer
}

func newLED(r *Radio, pin uint8) *led {
	return &led{r: r, pin: pin}
}

// trigger lights the LED and (re)arms its auto-off timer for ledHold.
func (l *led) trigger() {
	l.r.setGPIO(l.pin, true)
	l.mu.Lock()
	if l.off == nil {
		l.off = time.AfterFunc(ledHold, func() { l.r.setGPIO(l.pin, false) })
	} else {
		l.off.Reset(ledHold)
	}
	l.mu.Unlock()
}

// stop cancels the auto-off timer and turns the LED off immediately.
func (l *led) stop() {
	l.mu.Lock()
	if l.off != nil {
		l.off.Stop()
	}
	l.mu.Unlock()
	l.r.setGPIO(l.pin, false)
}

// setGPIO drives a dongle GPIO pin under the shared USB lock. LED errors are
// non-fatal: a missed status-LED update must never break radio traffic.
func (r *Radio) setGPIO(pin uint8, on bool) {
	r.mu.Lock()
	_ = r.dev.SetGPIO(pin, on)
	r.mu.Unlock()
}
