package node

import "testing"

func TestNewDescriptor(t *testing.T) {
	regs := []Register{
		{ID: 1, Name: "green", Type: TypeInt},
		{ID: 2, Name: "red", Type: TypeInt},
	}

	d, err := NewDescriptor(map[string]string{"device": "bob"}, regs)
	if err != nil {
		t.Fatalf("NewDescriptor: %v", err)
	}

	if r, ok := d.ByID(1); !ok || r.Name != "green" {
		t.Fatalf("ByID(1) = %v, %v", r, ok)
	}
	if r, ok := d.ByName("red"); !ok || r.ID != 2 {
		t.Fatalf("ByName(red) = %v, %v", r, ok)
	}
}

func TestNewDescriptorRejectsBadTable(t *testing.T) {
	cases := []struct {
		name string
		regs []Register
	}{
		{"zero id", []Register{{ID: 0, Name: "a", Type: TypeBool}}},
		{"duplicate id", []Register{
			{ID: 1, Name: "a", Type: TypeBool},
			{ID: 1, Name: "b", Type: TypeBool},
		}},
		{"duplicate name", []Register{
			{ID: 1, Name: "a", Type: TypeBool},
			{ID: 2, Name: "a", Type: TypeBool},
		}},
		{"incomplete writable conversion", []Register{{ID: 1, Name: "a", Type: TypeFloat, Conversion: Conversion{Decode: func(raw int32) (any, error) { return raw, nil }}}}},
		{"encoder on read-only register", []Register{{ID: 1, Name: "a", Type: TypeFloat, ReadOnly: true, Conversion: Conversion{Encode: func(value any) (int32, error) { return 0, nil }}}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NewDescriptor(nil, c.regs); err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
		})
	}
}
