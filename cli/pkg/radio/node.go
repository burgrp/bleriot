package radio

// NodeRadio adapts a Dongle to protocol/node.Radio, the node (firmware) end of
// the link. Unlike the hub-side Radio it runs no receive goroutine: the node
// runtime drives Receive synchronously from its Poll loop. Running it on the
// host (over an mcpdongle) lets the machine-free node runtime be exercised over
// real RF for functional tests without any microcontroller.
type NodeRadio struct {
	d Dongle
}

// NewNode wraps d as the node end of the link. The dongle must already be
// brought up on the node's receive address.
func NewNode(d Dongle) *NodeRadio {
	return &NodeRadio{d: d}
}

// Send transmits one packet to dst.
func (r *NodeRadio) Send(dst [4]byte, packet []byte) error {
	return r.d.Send(dst, packet)
}

// Receive copies at most one received packet into buf and reports how many bytes
// were written and whether a packet was available. It never blocks.
func (r *NodeRadio) Receive(buf []byte) (int, bool) {
	return r.d.Receive(buf)
}

// Close releases the underlying dongle (turning its status LEDs off and closing
// the device).
func (r *NodeRadio) Close() error {
	return r.d.Close()
}
