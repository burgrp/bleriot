package node

import "testing"

func TestNewDescriptor(t *testing.T) {
	regs := []Register{
		{ID: 1, Name: "green", Type: TypeInt, Multiplier: 1, Divider: 1},
		{ID: 2, Name: "red", Type: TypeInt, Multiplier: 1, Divider: 1},
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
		{"zero divider", []Register{{ID: 1, Name: "a", Type: TypeFloat, Multiplier: 1}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := NewDescriptor(nil, c.regs); err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
		})
	}
}
