// Package descriptor defines the host-side authoring model for BleRiot
// registers and the deterministic wire-ID allocation used by the code
// generator. See the protocol spec §11.
//
// This package is host tooling only. It is never compiled into node firmware:
// node firmware receives generated code (const wire IDs + interfaces + wiring
// table), not these descriptors. Class descriptors and node specs intentionally
// carry no wire IDs — IDs are assigned exclusively by AllocateIDs (§11.6).
package descriptor

// RegType is the hub-side interpretation hint for a register's int32 wire value
// (§11.3). The node always transmits raw int32 regardless of type.
type RegType string

const (
	TypeInt   RegType = "int"   // no scaling
	TypeFloat RegType = "float" // display = wire × multiplier / divider
	TypeBool  RegType = "bool"  // 0 = false, 1 = true; scaling ignored
)

// RegisterDescriptor describes a single register within a class (§11.3).
// It has no wire ID; the generator assigns one per qualified name (§11.6).
type RegisterDescriptor struct {
	Name       string            // unique within its class, e.g. "temperature"
	Type       RegType           // int, float, or bool
	Multiplier int32             // hub scaling numerator
	Divider    int32             // hub scaling denominator; must not be zero
	Metadata   map[string]string // merged into the hub's register record
}

// ClassDescriptor is a reusable, named set of registers — a register profile
// (§11.2). Register names are unique within the class.
type ClassDescriptor struct {
	Name      string               // unique class name, e.g. "thermometer"
	Registers []RegisterDescriptor // register profile
	Metadata  map[string]string    // merged into every instance's hub record
}

// ClassInstance composes a class onto a node under a node-unique instance name
// (§11.4). The qualified register name is Name + "." + register name.
type ClassInstance struct {
	Class string // name of a ClassDescriptor
	Name  string // instance name, unique within the node, e.g. "outdoor"
}

// NodeSpec describes a concrete node-type as a composition of class instances
// (§11.4). It carries no per-device provisioning data: address, key, and RF
// channel are all assigned per device at provisioning time (§11.5), not here.
type NodeSpec struct {
	Name      string            // node-type name; names the generated artifacts (not a per-device node name)
	Metadata  map[string]string // merged into the hub's node record
	Instances []ClassInstance   // class instances composed onto this node
}
