package node

import "github.com/burgrp/bleriot/lib/shared/config"

// Provisioning is a node's per-device identity, baked into the firmware image by
// the host "make" command; "gen" emits the same source for inspection (see
// lib/site/cli). The generated main() constructs one of these and hands it to
// the firmware's bleriotMain entry point together with the device config.
type Provisioning struct {
	// Address is the node's random, nonzero 4-byte RF address (§3).
	Address [config.AddrLen]byte
	// Key is the node's 16-byte XTEA shared key (§5); secret.
	Key [config.KeyLen]byte
	// Channel is the raw RF channel (0..83) the node listens and transmits on.
	Channel uint8
	// SpreadFactor is the BLE Coded PHY spreading factor for the channel.
	SpreadFactor config.SpreadFactor
}
