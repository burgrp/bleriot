//go:build tinygo

package thermostat

import "protocol/node"

// thermostat is the running device. It holds the live register state and a
// reference to the runtime so it can push updates when a value changes on its
// own. The struct is allocated once in Start; the run loop never allocates.
type thermostat struct {
	cfg Config

	temperature int32 // measured temperature, ×100 (wire units)
	setpoint    int32 // target temperature, ×100 (wire units)
	setpointSet bool  // false = thermostat off (setpoint NULL), heating forced off
	heating     bool  // heating element state

	rt *node.Node
}

// Start brings the thermostat firmware up: it builds the protocol runtime around
// the provisioned identity (address, key) and the supplied radio, then runs the
// dispatch loop forever. radio must already be configured for channel.
func Start(address [4]byte, key [16]byte, channel uint8, cfg Config, radio node.Radio) error {
	println("Thermostat starting...")

	// The thermostat starts off: the setpoint is NULL until a hub sets one.
	t := &thermostat{cfg: cfg}

	rt, err := node.New(radio, address, key, t)
	if err != nil {
		return err
	}
	t.rt = rt

	rt.Run() // never returns
	return nil
}

// Read returns the current value of a register by tag (PROTOCOL.md GET). It is
// the device's read switch: one case per register.
func (t *thermostat) Read(tag uint16) (int32, bool) {
	switch tag {
	case TagTemperature:
		return t.temperature, false
	case TagSetpoint:
		// A NULL setpoint means the thermostat is off.
		return t.setpoint, !t.setpointSet
	case TagHeating:
		return boolToWire(t.heating), false
	default:
		return 0, true // unknown register: no value
	}
}

// Write applies a value to a register by tag (PROTOCOL.md SET). It is the
// device's write switch. It has no return value; the runtime acknowledges the
// SET with an ACK. Writes to read-only or unknown registers are ignored. A NULL
// setpoint turns the thermostat off (heating forced off).
func (t *thermostat) Write(tag uint16, value int32, null bool) {
	switch tag {
	case TagSetpoint:
		if null {
			// Clear the setpoint: thermostat off.
			t.setpointSet = false
			t.control()
			return
		}
		// Clamp the requested setpoint to the configured operating range.
		lo := int32(t.cfg.MinTemp * 100)
		hi := int32(t.cfg.MaxTemp * 100)
		if value < lo {
			value = lo
		} else if value > hi {
			value = hi
		}
		t.setpoint = value
		t.setpointSet = true
		t.control()
	}
}

// control updates the heating element from temperature vs setpoint and pushes an
// IS to any hub watching the heating register when it changes. With no setpoint
// (thermostat off) the heating is forced off.
func (t *thermostat) control() {
	want := t.setpointSet && t.temperature < t.setpoint
	if want != t.heating {
		t.heating = want
		t.rt.Notify(TagHeating, boolToWire(t.heating))
	}
}

func boolToWire(b bool) int32 {
	if b {
		return 1
	}
	return 0
}
