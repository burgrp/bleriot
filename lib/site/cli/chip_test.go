package cli

import (
	"testing"

	"github.com/burgrp/bleriot/lib/shared/config"
	"github.com/burgrp/bleriot/lib/shared/inventory"
)

var altChip = inventory.Chip{Name: "stm32g0", Target: "stm32g030x6", UIDAddr: 0x1FFF7590}

func TestResolveChip(t *testing.T) {
	t.Run("auto-selects sole inventory chip", func(t *testing.T) {
		inv := inventory.Inventory{sampleInstance()} // PY32F030x8
		c, err := resolveChip(inv, "")
		if err != nil {
			t.Fatalf("resolveChip: %v", err)
		}
		if c != inventory.PY32F030x8 {
			t.Fatalf("got %+v, want PY32F030x8", c)
		}
	})

	t.Run("errors on empty inventory without --chip", func(t *testing.T) {
		if _, err := resolveChip(inventory.Inventory{}, ""); err == nil {
			t.Fatal("expected error for empty inventory")
		}
	})

	t.Run("built-in selectable by name on empty inventory", func(t *testing.T) {
		c, err := resolveChip(inventory.Inventory{}, "py32f030x8")
		if err != nil {
			t.Fatalf("resolveChip: %v", err)
		}
		if c != inventory.PY32F030x8 {
			t.Fatalf("got %+v, want PY32F030x8", c)
		}
	})

	t.Run("unknown name errors", func(t *testing.T) {
		if _, err := resolveChip(inventory.Inventory{sampleInstance()}, "nope"); err == nil {
			t.Fatal("expected error for unknown chip")
		}
	})

	t.Run("multiple chips require --chip", func(t *testing.T) {
		alt := sampleInstance()
		alt.Name = "other"
		alt.UID = [config.UIDLen]byte{0x99}
		alt.Type.Chip = altChip
		inv := inventory.Inventory{sampleInstance(), alt}

		if _, err := resolveChip(inv, ""); err == nil {
			t.Fatal("expected error: multiple chips need --chip")
		}
		c, err := resolveChip(inv, "stm32g0")
		if err != nil {
			t.Fatalf("resolveChip(stm32g0): %v", err)
		}
		if c != altChip {
			t.Fatalf("got %+v, want altChip", c)
		}
	})
}
