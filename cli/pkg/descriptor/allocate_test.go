package descriptor

import "testing"

func library() map[string]ClassDescriptor {
	return map[string]ClassDescriptor{
		"thermometer": {
			Name: "thermometer",
			Registers: []RegisterDescriptor{
				{Name: "temperature", Type: TypeFloat, Multiplier: 1, Divider: 100, Metadata: map[string]string{"unit": "celsius"}},
				{Name: "humidity", Type: TypeInt, Multiplier: 1, Divider: 1},
			},
		},
		"switch": {
			Name:      "switch",
			Registers: []RegisterDescriptor{{Name: "relay", Type: TypeBool}},
		},
	}
}

// §11.8 worked example.
func garageSpec() NodeSpec {
	return NodeSpec{
		Name: "garage-controller",
		Instances: []ClassInstance{
			{Class: "thermometer", Name: "outdoor"},
			{Class: "switch", Name: "main"},
			{Class: "switch", Name: "aux"},
		},
	}
}

func TestAllocateIDs_CanonicalOrderAndUniqueness(t *testing.T) {
	res, err := AllocateIDs(garageSpec(), library())
	if err != nil {
		t.Fatalf("AllocateIDs: %v", err)
	}

	wantNames := []string{"aux.relay", "main.relay", "outdoor.humidity", "outdoor.temperature"}
	if len(res.Registers) != len(wantNames) {
		t.Fatalf("got %d registers, want %d", len(res.Registers), len(wantNames))
	}
	seen := map[uint16]bool{}
	for i, r := range res.Registers {
		if r.Name != wantNames[i] {
			t.Errorf("register %d name = %q, want %q", i, r.Name, wantNames[i])
		}
		if r.ID == 0 {
			t.Errorf("register %q got reserved ID 0x0000", r.Name)
		}
		if seen[r.ID] {
			t.Errorf("duplicate ID 0x%04X for %q", r.ID, r.Name)
		}
		seen[r.ID] = true
	}
}

func TestAllocateIDs_OrderIndependent(t *testing.T) {
	a, err := AllocateIDs(garageSpec(), library())
	if err != nil {
		t.Fatal(err)
	}

	// Same node, instances declared in a different order.
	reordered := garageSpec()
	reordered.Instances = []ClassInstance{
		{Class: "switch", Name: "aux"},
		{Class: "switch", Name: "main"},
		{Class: "thermometer", Name: "outdoor"},
	}
	b, err := AllocateIDs(reordered, library())
	if err != nil {
		t.Fatal(err)
	}

	if a.Version != b.Version {
		t.Errorf("version hash changed under reordering: 0x%08X vs 0x%08X", a.Version, b.Version)
	}
	if len(a.Registers) != len(b.Registers) {
		t.Fatalf("register count differs: %d vs %d", len(a.Registers), len(b.Registers))
	}
	for i := range a.Registers {
		if a.Registers[i].Name != b.Registers[i].Name || a.Registers[i].ID != b.Registers[i].ID {
			t.Errorf("entry %d differs: %q=0x%04X vs %q=0x%04X", i,
				a.Registers[i].Name, a.Registers[i].ID, b.Registers[i].Name, b.Registers[i].ID)
		}
	}
}

func TestAllocateIDs_Deterministic(t *testing.T) {
	a, _ := AllocateIDs(garageSpec(), library())
	b, _ := AllocateIDs(garageSpec(), library())
	if a.Version != b.Version {
		t.Fatalf("non-deterministic version hash: 0x%08X vs 0x%08X", a.Version, b.Version)
	}
	for i := range a.Registers {
		if a.Registers[i].ID != b.Registers[i].ID {
			t.Fatalf("non-deterministic ID for %q", a.Registers[i].Name)
		}
	}
}

func TestAllocateIDs_Errors(t *testing.T) {
	lib := library()

	dup := garageSpec()
	dup.Instances = append(dup.Instances, ClassInstance{Class: "switch", Name: "main"})
	if _, err := AllocateIDs(dup, lib); err == nil {
		t.Error("expected error for duplicate instance name")
	}

	unknown := NodeSpec{Name: "x", Instances: []ClassInstance{{Class: "nope", Name: "i"}}}
	if _, err := AllocateIDs(unknown, lib); err == nil {
		t.Error("expected error for unknown class")
	}

	badLib := map[string]ClassDescriptor{
		"bad": {Name: "bad", Registers: []RegisterDescriptor{{Name: "x", Type: TypeFloat, Divider: 0}}},
	}
	zeroDiv := NodeSpec{Name: "x", Instances: []ClassInstance{{Class: "bad", Name: "i"}}}
	if _, err := AllocateIDs(zeroDiv, badLib); err == nil {
		t.Error("expected error for zero divider on non-bool register")
	}
}

func TestAllocateIDs_MetadataMerge(t *testing.T) {
	lib := map[string]ClassDescriptor{
		"c": {
			Name:     "c",
			Metadata: map[string]string{"vendor": "acme", "unit": "raw"},
			Registers: []RegisterDescriptor{
				{Name: "r", Type: TypeInt, Multiplier: 1, Divider: 1, Metadata: map[string]string{"unit": "celsius"}},
			},
		},
	}
	res, err := AllocateIDs(NodeSpec{Name: "n", Instances: []ClassInstance{{Class: "c", Name: "i"}}}, lib)
	if err != nil {
		t.Fatal(err)
	}
	md := res.Registers[0].Metadata
	if md["vendor"] != "acme" {
		t.Errorf("class metadata not inherited: %v", md)
	}
	if md["unit"] != "celsius" {
		t.Errorf("register metadata should override class: got %q", md["unit"])
	}
}
