package thermostat

// Register tags are the thermostat's permanent wire identities (PROTOCOL.md §11):
// unique and non-zero within the device type, and never reused once retired.
//
// This block is the single source of truth for those tags. The host register
// table (type.go) and the firmware register handlers (device.go) both reference
// these constants, so the two sides can never drift. They are untyped so they
// fit both inventory.Register.Tag (uint8) and the firmware runtime's uint16.
const (
	TagTemperature = 1 // measured temperature (float, ×0.01 °C)
	TagSetpoint    = 2 // target temperature (float, ×0.01 °C)
	TagHeating     = 3 // heating element on/off (bool)
)
