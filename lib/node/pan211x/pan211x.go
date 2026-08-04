package pan211x

import (
	"machine"

	"github.com/burgrp/bleriot/lib/node"
	"github.com/burgrp/bleriot/lib/shared/protocol"
	"github.com/burgrp/tinygo-drivers/bb/spi"
	"github.com/burgrp/tinygo-drivers/pan211x"
)

// StartNode brings up a Bleriot node backed by a PAN211x BLE long-range radio.
//
// It takes the node's identity (RF channel, spread factor, address, and XTEA
// key) as a node.Provisioning value baked into the firmware image by the host
// "gen" command, initializes the radio over a 3-wire SPI interface on the given
// pins, tunes it to the provisioned channel, and registers the node's receive
// address. On success it returns the constructed node; any failure during radio
// setup is returned as an error.
func StartNode(prov node.Provisioning, pinSpiSck, pinSpiData, pinSpiCsn machine.Pin, device node.Device) (*node.Node, error) {

	println("Starting Bleriot node with PAN211x radio...")
	println("Provisioned channel", int(prov.Channel), ", spreadFactor", prov.SpreadFactor.String())

	radio := pan211x.NewDriverBLELongRange(pan211x.NewRegistersSPI(spi.NewMaster(pinSpiSck, pinSpiData), pinSpiCsn))

	err := radio.Init(pan211x.ConfigBLELongRange{
		PayloadLen:      protocol.PacketLen,
		SerialInterface: pan211x.SerialInterfaceSPI3W,
		SpreadFactor:    pan211x.SpreadFactor(prov.SpreadFactor),
	})
	if err != nil {
		return nil, err
	}

	err = radio.SetChannelRF(prov.Channel, prov.Channel)
	if err != nil {
		return nil, err
	}

	err = radio.EnableRxAddress(0, prov.Address)
	if err != nil {
		return nil, err
	}

	return node.New(radio, prov.Address, prov.Key, device)
}
