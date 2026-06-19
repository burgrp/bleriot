package inventory

import (
	"testing"

	"site/pkg/config"
)

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
		{Name: "kitchen", Channel: Channel{Number: 37}, Type: thermostatType()},
		{Name: "living", Channel: Channel{Number: 11}, Type: thermostatType()},
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
	t.Run("mixed spread factor on one channel", func(t *testing.T) {
		inv := Inventory{
			{Name: "kitchen", Channel: Channel{Number: 37, SpreadFactor: config.SpreadFactorS8}, Type: thermostatType()},
			{Name: "living", Channel: Channel{Number: 37, SpreadFactor: config.SpreadFactorS2}, Type: thermostatType()},
		}
		if err := inv.Validate(); err == nil {
			t.Fatal("expected error for mixed spread factor on one channel")
		}
	})
}

func TestSpreadFactorByChannel(t *testing.T) {
	inv := Inventory{
		{Name: "kitchen", Channel: Channel{Number: 37, SpreadFactor: config.SpreadFactorS2}, Type: thermostatType()},
		{Name: "hallway", Channel: Channel{Number: 37, SpreadFactor: config.SpreadFactorS2}, Type: thermostatType()},
		{Name: "lab", Channel: Channel{Number: 11, SpreadFactor: config.SpreadFactorS8}, Type: thermostatType()},
		{Name: "shed", Channel: Channel{Number: 5}, Type: thermostatType()}, // omitted SF: defaults to S8
	}
	byChannel, err := inv.SpreadFactorByChannel()
	if err != nil {
		t.Fatalf("SpreadFactorByChannel: %v", err)
	}
	want := map[uint8]config.SpreadFactor{37: config.SpreadFactorS2, 11: config.SpreadFactorS8, 5: config.SpreadFactorS8}
	if len(byChannel) != len(want) {
		t.Fatalf("got %d channels, want %d: %v", len(byChannel), len(want), byChannel)
	}
	for ch, sf := range want {
		if byChannel[ch] != sf {
			t.Errorf("channel %d: got spread factor %d, want %d", ch, byChannel[ch], sf)
		}
	}
}

func TestSpreadFactorByChannel_Conflict(t *testing.T) {
	inv := Inventory{
		{Name: "kitchen", Channel: Channel{Number: 37, SpreadFactor: config.SpreadFactorS8}, Type: thermostatType()},
		{Name: "living", Channel: Channel{Number: 37, SpreadFactor: config.SpreadFactorS2}, Type: thermostatType()},
	}
	if _, err := inv.SpreadFactorByChannel(); err == nil {
		t.Fatal("expected conflict error for mixed spread factor on one channel")
	}
}
