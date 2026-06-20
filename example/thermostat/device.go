//go:build tinygo

package main

import (
	"protocol/node"

	"thermostat/spec"
)

// Device is the running thermostat: the application behind the registers. It
// holds the live register state and a reference to the runtime so it can push
// updates when a value changes on its own (PROTOCOL.md §8, WATCH).
//
// It implements node.Device (Read/Write) and is otherwise driven by the firmware
// main loop, which feeds it temperature samples (UpdateTemperature) and reads
// back the heating decision (Heating) to drive the relay. The struct is
// allocated once at boot; the run loop never allocates.
type Device struct {
	cfg spec.Config

	temperature int32 // measured temperature, ×100 (wire units)
	setpoint    int32 // target temperature, ×100 (wire units)
	setpointSet bool  // false = thermostat off (setpoint NULL), heating forced off
	heating     bool  // heating element state

	rt *node.Node
}

// New builds a thermostat device for the given configuration. The thermostat
// starts off: the setpoint is NULL until a hub sets one. Call Attach with the
// runtime before driving the device so it can push IS updates to watchers.
func New(cfg spec.Config) *Device {
	return &Device{cfg: cfg}
}

// Attach wires the device to its protocol runtime so control changes can be
// pushed to watching hubs via Notify.
func (d *Device) Attach(rt *node.Node) { d.rt = rt }

// Read returns the current value of a register by tag (PROTOCOL.md GET). It is
// the device's read switch: one case per register.
func (d *Device) Read(tag uint16) (int32, bool) {
	switch tag {
	case spec.TagTemperature:
		return d.temperature, false
	case spec.TagSetpoint:
		// A NULL setpoint means the thermostat is off.
		return d.setpoint, !d.setpointSet
	case spec.TagHeating:
		return boolToWire(d.heating), false
	default:
		return 0, true // unknown register: no value
	}
}

// Write applies a value to a register by tag (PROTOCOL.md SET). It is the
// device's write switch. It has no return value; the runtime acknowledges the
// SET with an ACK. Writes to read-only or unknown registers are ignored. A NULL
// setpoint turns the thermostat off (heating forced off).
func (d *Device) Write(tag uint16, value int32, null bool) {
	switch tag {
	case spec.TagSetpoint:
		if null {
			// Clear the setpoint: thermostat off. Push the now-NULL setpoint to
			// any watcher so the off state is visible without a fresh GET.
			d.setpointSet = false
			d.rt.Notify(spec.TagSetpoint, 0, true)
			d.control()
			return
		}
		// Clamp the requested setpoint to the configured operating range.
		lo := int32(d.cfg.MinTemp * 100)
		hi := int32(d.cfg.MaxTemp * 100)
		if value < lo {
			value = lo
		} else if value > hi {
			value = hi
		}
		d.setpoint = value
		d.setpointSet = true
		d.rt.Notify(spec.TagSetpoint, value, false)
		d.control()
	}
}

// UpdateTemperature records a fresh temperature sample (×100 wire units) from the
// sensor, re-evaluates the heating decision, and pushes the new reading to any
// hub watching the temperature register. The firmware main loop calls this
// periodically.
func (d *Device) UpdateTemperature(value int32) {
	if value == d.temperature {
		return
	}
	d.temperature = value
	d.rt.Notify(spec.TagTemperature, value, false)
	d.control()
}

// Heating reports the current heating-element decision so the firmware can drive
// the relay output.
func (d *Device) Heating() bool { return d.heating }

// control updates the heating element from temperature vs setpoint and pushes an
// IS to any hub watching the heating register when it changes. With no setpoint
// (thermostat off) the heating is forced off.
func (d *Device) control() {
	want := d.setpointSet && d.temperature < d.setpoint
	if want != d.heating {
		d.heating = want
		d.rt.Notify(spec.TagHeating, boolToWire(d.heating), false)
	}
}

func boolToWire(b bool) int32 {
	if b {
		return 1
	}
	return 0
}
