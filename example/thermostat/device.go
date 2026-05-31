//go:build tinygo

package thermostat

func Start(address [4]byte, key [16]byte, channel uint8, cfg Config) {
	println("Thermostat starting...")
	println("Min Temperature:", cfg.MinTemp)
	println("Max Temperature:", cfg.MaxTemp)
}
