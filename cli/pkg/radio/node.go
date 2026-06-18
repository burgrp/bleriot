package radio

import (
	"github.com/burgrp/tinygo-drivers/pan211x"

	"cli/pkg/mcp2210"
)

// NodeRadio drives a PAN211x as the node (firmware) end of the link, run on the
// host through an MCP2210 dongle. It satisfies protocol/node.Radio, so the
// machine-free node runtime can be exercised over real RF for functional tests
// without any microcontroller.
type NodeRadio struct {
	*dongle
}

// NewNode initialises a dongle's radio for BLE LongRange on the given channel
// and node receive address (so packets the hub sends to this node are received).
// Unlike the hub Radio it runs no receive goroutine: the node runtime drives
// Receive synchronously from its Poll loop. The caller owns dev and must Close
// the NodeRadio (to turn the LEDs off) and the device when done.
func NewNode(dev *mcp2210.Device, channel uint8, nodeAddr pan211x.AddressBLE) (*NodeRadio, error) {
	d, err := openDongle(dev, channel, nodeAddr)
	if err != nil {
		return nil, err
	}
	return &NodeRadio{dongle: d}, nil
}

// Send transmits one packet to dst.
func (r *NodeRadio) Send(dst [4]byte, packet []byte) error {
	return r.send(dst, packet)
}

// Receive copies at most one received packet into buf and reports how many bytes
// were written and whether a packet was available. It never blocks.
func (r *NodeRadio) Receive(buf []byte) (int, bool) {
	return r.receive(buf)
}

// Close turns the status LEDs off and cancels their timers. It does not close
// the underlying device.
func (r *NodeRadio) Close() {
	r.stopLEDs()
}
