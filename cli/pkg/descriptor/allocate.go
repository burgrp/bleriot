package descriptor

import (
	"errors"
	"fmt"
	"sort"
)

// ResolvedRegister is one entry of a generated node descriptor (§11.7): a
// qualified register name with its allocated wire ID and resolved hub metadata.
type ResolvedRegister struct {
	ID         uint16
	Name       string // qualified name: instance + "." + register
	Class      string
	Instance   string
	Type       RegType
	Multiplier int32
	Divider    int32
	Metadata   map[string]string
}

// Resolved is the full output of resolving a NodeSpec against a class library:
// the flat register list (sorted by qualified name) plus the descriptor ID
// (§11.6 step 5) used for firmware/descriptor mismatch detection.
type Resolved struct {
	Node      string
	Metadata  map[string]string
	Version   uint32
	Registers []ResolvedRegister
}

// AllocateIDs resolves a NodeSpec against the given class library and assigns a
// deterministic uint16 wire ID to every qualified register name, per the
// algorithm in the protocol spec §11.6:
//
//  1. Collect all qualified names across every instance.
//  2. Sort lexicographically (canonical order — independent of authoring order).
//  3. Primary slot = fnv1a32(qualifiedName) & 0xFFFF; 0x0000 is reserved.
//  4. On collision, linear probe (id+1)&0xFFFF, skipping 0x0000.
//  5. Compute a descriptor ID over the resolved tuples in canonical order.
//
// The library maps class name → ClassDescriptor. AllocateIDs returns an error
// for unknown classes, duplicate instance names, or a zero divider.
func AllocateIDs(spec NodeSpec, library map[string]ClassDescriptor) (Resolved, error) {
	seenInstance := make(map[string]bool, len(spec.Instances))
	var regs []ResolvedRegister

	for _, inst := range spec.Instances {
		if seenInstance[inst.Name] {
			return Resolved{}, fmt.Errorf("duplicate instance name %q", inst.Name)
		}
		seenInstance[inst.Name] = true

		class, ok := library[inst.Class]
		if !ok {
			return Resolved{}, fmt.Errorf("instance %q references unknown class %q", inst.Name, inst.Class)
		}

		for _, r := range class.Registers {
			if r.Type != TypeBool && r.Divider == 0 {
				return Resolved{}, fmt.Errorf("register %s.%s has zero divider", inst.Name, r.Name)
			}
			regs = append(regs, ResolvedRegister{
				Name:       inst.Name + "." + r.Name,
				Class:      inst.Class,
				Instance:   inst.Name,
				Type:       r.Type,
				Multiplier: r.Multiplier,
				Divider:    r.Divider,
				Metadata:   mergeMetadata(class.Metadata, r.Metadata),
			})
		}
	}

	// Canonical order: sort by qualified name (§11.6 step 2).
	sort.Slice(regs, func(i, j int) bool { return regs[i].Name < regs[j].Name })

	// Allocate slots with linear probing (§11.6 steps 3–4).
	taken := make(map[uint16]bool, len(regs))
	if len(regs) > 0xFFFF {
		return Resolved{}, errors.New("too many registers for 16-bit ID space")
	}
	for i := range regs {
		id := uint16(fnv1a32(regs[i].Name) & 0xFFFF)
		for id == 0 || taken[id] {
			id++ // wraps at 0xFFFF→0x0000, then skipped by the id==0 guard
		}
		taken[id] = true
		regs[i].ID = id
	}

	return Resolved{
		Node:      spec.Name,
		Metadata:  spec.Metadata,
		Version:   versionHash(regs),
		Registers: regs,
	}, nil
}

// mergeMetadata returns class metadata overlaid with register metadata. Register
// keys win. The result is always non-nil.
func mergeMetadata(class, reg map[string]string) map[string]string {
	out := make(map[string]string, len(class)+len(reg))
	for k, v := range class {
		out[k] = v
	}
	for k, v := range reg {
		out[k] = v
	}
	return out
}

const (
	fnvOffset32 uint32 = 0x811c9dc5
	fnvPrime32  uint32 = 0x01000193
)

// fnv1a32 is the 32-bit FNV-1a hash of s.
func fnv1a32(s string) uint32 {
	h := fnvOffset32
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= fnvPrime32
	}
	return h
}

// versionHash hashes the resolved tuples in canonical order (§11.6 step 5),
// covering id, qualified name, type, multiplier, and divider.
func versionHash(regs []ResolvedRegister) uint32 {
	h := fnvOffset32
	mix := func(b byte) {
		h ^= uint32(b)
		h *= fnvPrime32
	}
	mixStr := func(s string) {
		for i := 0; i < len(s); i++ {
			mix(s[i])
		}
		mix(0)
	}
	mixU32 := func(v uint32) {
		mix(byte(v))
		mix(byte(v >> 8))
		mix(byte(v >> 16))
		mix(byte(v >> 24))
	}
	for _, r := range regs {
		mixU32(uint32(r.ID))
		mixStr(r.Name)
		mixStr(string(r.Type))
		mixU32(uint32(r.Multiplier))
		mixU32(uint32(r.Divider))
	}
	return h
}
