package inventory

import (
	"testing"

	"github.com/burgrp/bleriot/lib/shared/config"
)

func bobType() DeviceType {
	return DeviceType{
		Name: "bob",
		Registers: []Register{
			{Tag: 1, Name: "green", Type: TypeInt},
			{Tag: 2, Name: "red", Type: TypeInt},
			{Tag: 3, Name: "gpio", Type: TypeInt},
		},
	}
}

func TestDeviceTypeValidate_OK(t *testing.T) {
	if err := bobType().Validate(); err != nil {
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
		{"incomplete conversion", DeviceType{Name: "t", Registers: []Register{
			{Tag: 1, Name: "a", Type: TypeFloat, ToValue: func(wire int32) any { return wire }},
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
		{Name: "kitchen", Address: [config.AddrLen]byte{1}, Channel: Channel{Name: "far", Number: 37}, Type: bobType()},
		{Name: "living", Address: [config.AddrLen]byte{2}, Channel: Channel{Name: "near", Number: 11}, Type: bobType()},
	}
	if err := inv.Validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestInventoryValidate_Errors(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		inv := Inventory{{Name: "", Channel: Channel{Name: "far", Number: 37}, Type: bobType()}}
		if err := inv.Validate(); err == nil {
			t.Fatal("expected error for empty instance name")
		}
	})
	t.Run("duplicate name", func(t *testing.T) {
		inv := Inventory{
			{Name: "kitchen", Address: [config.AddrLen]byte{1}, Channel: Channel{Name: "far", Number: 37}, Type: bobType()},
			{Name: "kitchen", Address: [config.AddrLen]byte{2}, Channel: Channel{Name: "far", Number: 37}, Type: bobType()},
		}
		if err := inv.Validate(); err == nil {
			t.Fatal("expected error for duplicate instance name")
		}
	})
	t.Run("reserved zero address", func(t *testing.T) {
		inv := Inventory{{Name: "kitchen", Channel: Channel{Name: "far", Number: 37}, Type: bobType()}}
		if err := inv.Validate(); err == nil {
			t.Fatal("expected error for zero address")
		}
	})
	t.Run("duplicate address", func(t *testing.T) {
		inv := Inventory{
			{Name: "kitchen", Address: [config.AddrLen]byte{1}, Channel: Channel{Name: "far", Number: 37}, Type: bobType()},
			{Name: "living", Address: [config.AddrLen]byte{1}, Channel: Channel{Name: "far", Number: 37}, Type: bobType()},
		}
		if err := inv.Validate(); err == nil {
			t.Fatal("expected error for duplicate address")
		}
	})
	t.Run("bad device type propagates", func(t *testing.T) {
		inv := Inventory{
			{Name: "kitchen", Address: [config.AddrLen]byte{1}, Channel: Channel{Name: "far", Number: 37}, Type: DeviceType{Name: "t", Registers: []Register{{Tag: 0, Name: "a", Type: TypeBool}}}},
		}
		if err := inv.Validate(); err == nil {
			t.Fatal("expected error from invalid device type")
		}
	})
	t.Run("missing channel name", func(t *testing.T) {
		inv := Inventory{
			{Name: "kitchen", Address: [config.AddrLen]byte{1}, Channel: Channel{Number: 37}, Type: bobType()},
		}
		if err := inv.Validate(); err == nil {
			t.Fatal("expected error for unnamed channel")
		}
	})
	t.Run("one number two names", func(t *testing.T) {
		inv := Inventory{
			{Name: "kitchen", Address: [config.AddrLen]byte{1}, Channel: Channel{Name: "far", Number: 37}, Type: bobType()},
			{Name: "living", Address: [config.AddrLen]byte{2}, Channel: Channel{Name: "near", Number: 37}, Type: bobType()},
		}
		if err := inv.Validate(); err == nil {
			t.Fatal("expected error for one channel number with two names")
		}
	})
	t.Run("one name two numbers", func(t *testing.T) {
		inv := Inventory{
			{Name: "kitchen", Address: [config.AddrLen]byte{1}, Channel: Channel{Name: "far", Number: 37}, Type: bobType()},
			{Name: "living", Address: [config.AddrLen]byte{2}, Channel: Channel{Name: "far", Number: 11}, Type: bobType()},
		}
		if err := inv.Validate(); err == nil {
			t.Fatal("expected error for one channel name with two numbers")
		}
	})
	t.Run("mixed spread factor on one channel", func(t *testing.T) {
		inv := Inventory{
			{Name: "kitchen", Address: [config.AddrLen]byte{1}, Channel: Channel{Name: "far", Number: 37, SpreadFactor: config.SpreadFactorS8}, Type: bobType()},
			{Name: "living", Address: [config.AddrLen]byte{2}, Channel: Channel{Name: "far", Number: 37, SpreadFactor: config.SpreadFactorS2}, Type: bobType()},
		}
		if err := inv.Validate(); err == nil {
			t.Fatal("expected error for mixed spread factor on one channel")
		}
	})
}

func TestSpreadFactorByChannel(t *testing.T) {
	inv := Inventory{
		{Name: "kitchen", Channel: Channel{Number: 37, SpreadFactor: config.SpreadFactorS2}, Type: bobType()},
		{Name: "hallway", Channel: Channel{Number: 37, SpreadFactor: config.SpreadFactorS2}, Type: bobType()},
		{Name: "lab", Channel: Channel{Number: 11, SpreadFactor: config.SpreadFactorS8}, Type: bobType()},
		{Name: "shed", Channel: Channel{Number: 5}, Type: bobType()}, // omitted SF: defaults to S8
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
		{Name: "kitchen", Channel: Channel{Number: 37, SpreadFactor: config.SpreadFactorS8}, Type: bobType()},
		{Name: "living", Channel: Channel{Number: 37, SpreadFactor: config.SpreadFactorS2}, Type: bobType()},
	}
	if _, err := inv.SpreadFactorByChannel(); err == nil {
		t.Fatal("expected conflict error for mixed spread factor on one channel")
	}
}
