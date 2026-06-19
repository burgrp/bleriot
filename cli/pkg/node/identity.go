package node

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"

	"cli/pkg/config"
)

// AddrLen is the BleRiot device address length in bytes (§3).
const AddrLen = 4

// KeyLen is the XTEA shared-key length in bytes (§5).
const KeyLen = 16

// AddressFromUID derives a node's 4-byte RF address from its 12-byte MCU unique
// ID (PROTOCOL.md §11.5): address = CRC32(UID), big-endian. Both the host
// (provisioning, hub) and the firmware compute it the same way, so the address
// is never stored in the inventory.
func AddressFromUID(uid [config.UIDLen]byte) [AddrLen]byte {
	var a [AddrLen]byte
	binary.BigEndian.PutUint32(a[:], crc32.ChecksumIEEE(uid[:]))
	return a
}

// Identity is a node's per-chip secret material, provisioned out of band
// (PROTOCOL.md §11.5) and never present in the generated descriptor.
type Identity struct {
	Address [AddrLen]byte
	Key     [KeyLen]byte
}

// ParseIdentity builds an Identity from a hex address (e.g. "0xA3F2B841" or
// "A3F2B841", big-endian as written) and a 32-char hex key.
func ParseIdentity(address, key string) (Identity, error) {
	var id Identity

	addr, err := ParseAddress(address)
	if err != nil {
		return Identity{}, fmt.Errorf("address: %w", err)
	}
	id.Address = addr

	k, err := hex.DecodeString(key)
	if err != nil {
		return Identity{}, fmt.Errorf("key: %w", err)
	}
	if len(k) != KeyLen {
		return Identity{}, fmt.Errorf("key: must be %d bytes, got %d", KeyLen, len(k))
	}
	copy(id.Key[:], k)
	return id, nil
}

// ParseAddress parses a BleRiot device address from hex (e.g. "0xA3F2B841" or
// "A3F2B841", big-endian as written). The optional "0x"/"0X" prefix is allowed.
func ParseAddress(s string) ([AddrLen]byte, error) {
	var a [AddrLen]byte
	if len(s) >= 2 && (s[:2] == "0x" || s[:2] == "0X") {
		s = s[2:]
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return a, err
	}
	if len(b) != AddrLen {
		return a, fmt.Errorf("must be %d bytes, got %d", AddrLen, len(b))
	}
	copy(a[:], b)
	return a, nil
}

// Node couples a node's descriptor with its provisioned identity, name, and RF
// channel. It is the host's complete view of one node. The name and channel
// identify and reach the physical device (on the hub they come from the
// instance file) and are distinct from the shared, per-type descriptor.
type Node struct {
	Name    string
	Channel uint8
	*Descriptor
	Identity
}

// NewNode pairs a descriptor with an identity under the given node name and RF
// channel.
func NewNode(name string, channel uint8, d *Descriptor, id Identity) *Node {
	return &Node{Name: name, Channel: channel, Descriptor: d, Identity: id}
}
