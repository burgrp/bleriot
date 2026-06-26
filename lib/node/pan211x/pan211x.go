package pan211x

import (
	"machine"
	"unsafe"

	"github.com/burgrp/bleriot/lib/node"
	"github.com/burgrp/bleriot/lib/shared/config"
	"github.com/burgrp/bleriot/lib/shared/inventory"
	"github.com/burgrp/bleriot/lib/shared/protocol"
	"github.com/burgrp/tinygo-drivers/bb/spi"
	"github.com/burgrp/tinygo-drivers/pan211x"
)

// StartNode brings up a Bleriot node backed by a PAN211x BLE long-range radio.
//
// It reads the provisioning page from the chip's configuration flash to obtain
// the RF channel, spread factor, node address, and key, initializes the radio
// over a 3-wire SPI interface on the given pins, tunes it to the provisioned
// channel, and registers the node's receive address. On success it returns the
// constructed node along with the raw device-config bytes from the page; any
// failure during decoding or radio setup is returned as an error.
func StartNode(chip *inventory.Chip, pinSpiSck, pinSpiData, pinSpiCsn machine.Pin, device node.Device) (*node.Node, []byte, error) {

	println("Starting Bleriot node with PAN211x radio...")

	pageData := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(chip.PageAddr))), chip.PageBytes)
	header, cfgBytes, err := config.Decode(pageData)
	if err != nil {
		return nil, nil, err
	}
	println("Provisioned channel", int(header.Channel), ", spreadFactor", int(header.SpreadFactor))

	radio := pan211x.NewDriverBLELongRange(pan211x.NewRegistersSPI(spi.NewMaster(pinSpiSck, pinSpiData), pinSpiCsn))

	err = radio.Init(pan211x.ConfigBLELongRange{
		PayloadLen:      protocol.PacketLen,
		SerialInterface: pan211x.SerialInterfaceSPI3W,
		SpreadFactor:    pan211x.SpreadFactor(header.SpreadFactor),
	})

	if err != nil {
		return nil, nil, err
	}

	err = radio.SetChannelRF(header.Channel, header.Channel)
	if err != nil {
		return nil, nil, err
	}

	err = radio.EnableRxAddress(0, header.Address)
	if err != nil {
		return nil, nil, err
	}

	node, err := node.New(radio, header.Address, header.Key, device)

	return node, cfgBytes, err
}
