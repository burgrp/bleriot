package cli

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/burgrp/bleriot/lib/shared/inventory"
)

// builtinChips are chip profiles the provisioning commands know without the
// inventory declaring them, so a brand-new deployment can onboard its first
// device with --chip before any instance exists.
var builtinChips = []inventory.Chip{
	inventory.PY32F030,
}

// chipCatalog collects every selectable chip, keyed by name: the built-ins plus
// every chip declared by the inventory's device types. Inventory chips win on
// name collision (the site's declaration is authoritative). It errors if one
// name maps to two different definitions within the inventory.
func chipCatalog(inv inventory.Inventory) (map[string]inventory.Chip, error) {
	chips := make(map[string]inventory.Chip)
	for _, c := range builtinChips {
		chips[c.Name] = c
	}
	for _, inst := range inv {
		c := inst.Type.Chip
		if c.Name == "" {
			continue
		}
		if existing, ok := chips[c.Name]; ok && existing != c && !isBuiltin(existing) {
			return nil, fmt.Errorf("chip %q is declared inconsistently across device types", c.Name)
		}
		chips[c.Name] = c
	}
	return chips, nil
}

func isBuiltin(c inventory.Chip) bool {
	for _, b := range builtinChips {
		if b == c {
			return true
		}
	}
	return false
}

// inventoryChips returns the distinct chips actually declared by the inventory's
// device types (excluding built-ins), keyed by name. This is what auto-selection
// considers, so an unused built-in never gets picked by accident.
func inventoryChips(inv inventory.Inventory) map[string]inventory.Chip {
	chips := make(map[string]inventory.Chip)
	for _, inst := range inv {
		if c := inst.Type.Chip; c.Name != "" {
			chips[c.Name] = c
		}
	}
	return chips
}

// resolveChip picks the chip the provisioning commands should drive over SWD. If
// name is given it selects that chip from the catalog (built-ins + inventory);
// otherwise it auto-selects the sole chip the inventory declares, erroring if
// the inventory declares zero or several.
func resolveChip(inv inventory.Inventory, name string) (inventory.Chip, error) {
	catalog, err := chipCatalog(inv)
	if err != nil {
		return inventory.Chip{}, err
	}
	if name != "" {
		c, ok := catalog[name]
		if !ok {
			return inventory.Chip{}, fmt.Errorf("unknown chip %q; known chips: %s", name, chipNames(catalog))
		}
		return c, nil
	}

	declared := inventoryChips(inv)
	switch len(declared) {
	case 1:
		for _, c := range declared {
			return c, nil
		}
	case 0:
		return inventory.Chip{}, fmt.Errorf(
			"no chip declared by any device type; set DeviceType.Chip (e.g. inventory.PY32F030) or pass --chip (known: %s)",
			chipNames(catalog))
	}
	return inventory.Chip{}, fmt.Errorf("inventory declares multiple chips (%s); select one with --chip", chipNames(declared))
}

// chipNames returns the catalog's chip names, sorted, for error messages.
func chipNames(chips map[string]inventory.Chip) string {
	names := make([]string, 0, len(chips))
	for n := range chips {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// chipProbe builds the hardware Probe that drives the given chip over SWD.
func chipProbe(c inventory.Chip, logger *slog.Logger) Probe {
	return &PyOCDProbe{
		Target:   c.Target,
		UIDAddr:  c.UIDAddr,
		PageAddr: c.PageAddr,
		Logger:   logger,
	}
}
