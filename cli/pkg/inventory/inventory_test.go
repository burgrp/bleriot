package inventory

import "testing"

func thermostatType() DeviceType {
	return DeviceType{
		Name: "thermostat",
		Registers: []Register{
			{Tag: 1, Name: "temperature", Type: TypeFloat, Multiplier: 1, Divider: 100},
			{Tag: 2, Name: "humidity", Type: TypeInt, Multiplier: 1, Divider: 1},
			{Tag: 3, Name: "heating", Type: TypeBool},
		},
	}
}

func TestDeviceTypeValidate_OK(t *testing.T) {
	if err := thermostatType().Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestDeviceTypeValidate_Errors(t *testing.T) {
	cases := []struct {
		name string
		dt   DeviceType
	}{
		{"no name", DeviceType{Registers: []Register{{Tag: 1, Name: "a", Type: TypeBool}}}},
		{"zero tag", DeviceType{Name: "t", Registers: []Register{{Tag: 0, Name: "a", Type: TypeBool}}}},
		{"dup tag", DeviceType{Name: "t", Registers: []Register{
			{Tag: 1, Name: "a", Type: TypeBool},
			{Tag: 1, Name: "b", Type: TypeBool},
		}}},
		{"empty reg name", DeviceType{Name: "t", Registers: []Register{{Tag: 1, Type: TypeBool}}}},
		{"dup reg name", DeviceType{Name: "t", Registers: []Register{
			{Tag: 1, Name: "a", Type: TypeBool},
			{Tag: 2, Name: "a", Type: TypeBool},
		}}},
		{"non-bool zero divider", DeviceType{Name: "t", Registers: []Register{
			{Tag: 1, Name: "a", Type: TypeFloat, Multiplier: 1, Divider: 0},
		}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.dt.Validate(); err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
		})
	}
}

func TestInventoryValidate_OK(t *testing.T) {
	inv := Inventory{
		{Name: "kitchen", Channel: 37, Type: thermostatType()},
		{Name: "living", Channel: 11, Type: thermostatType()},
	}
	if err := inv.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestInventoryValidate_Errors(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		inv := Inventory{{Name: "", Type: thermostatType()}}
		if err := inv.Validate(); err == nil {
			t.Fatal("expected error for empty instance name")
		}
	})
	t.Run("duplicate name", func(t *testing.T) {
		inv := Inventory{
			{Name: "kitchen", Type: thermostatType()},
			{Name: "kitchen", Type: thermostatType()},
		}
		if err := inv.Validate(); err == nil {
			t.Fatal("expected error for duplicate instance name")
		}
	})
	t.Run("bad device type propagates", func(t *testing.T) {
		inv := Inventory{
			{Name: "kitchen", Type: DeviceType{Name: "t", Registers: []Register{{Tag: 0, Name: "a", Type: TypeBool}}}},
		}
		if err := inv.Validate(); err == nil {
			t.Fatal("expected error from invalid device type")
		}
	})
}
