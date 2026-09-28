package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/burgrp/bleriot/lib/shared/config"
	"github.com/burgrp/bleriot/lib/shared/firmware"
	"github.com/burgrp/bleriot/lib/shared/inventory"
)

func TestRootCommandExposesNodeTreeOnly(t *testing.T) {
	root := newRootCmd(inventory.Inventory{sampleFirmwareInstance()})
	if findChild(root, "node") == nil {
		t.Fatal("root command has no node child")
	}
	for _, removed := range []string{"make", "gen", "new"} {
		if findChild(root, removed) != nil {
			t.Fatalf("root command still exposes removed %q child", removed)
		}
	}

	node := findChild(root, "node")
	for _, expected := range []string{"build", "gen", "new", "rtt", "gdb", "install-pack"} {
		if findChild(node, expected) == nil {
			t.Errorf("node command has no %q child", expected)
		}
	}
}

func TestNodeNameFlags(t *testing.T) {
	node := newNodeCmd(inventory.Inventory{sampleFirmwareInstance()})
	for _, childName := range []string{"build", "gen", "rtt", "gdb", "install-pack"} {
		child := findChild(node, childName)
		if child == nil {
			t.Fatalf("missing %q command", childName)
		}
		if child.Flags().Lookup("name") == nil {
			t.Errorf("%q command has no --name flag", childName)
		}
	}
	if child := findChild(node, "new"); child.Flags().Lookup("name") != nil {
		t.Fatal("node new unexpectedly has a --name flag")
	}
}

func TestResolveFirmwareInstance(t *testing.T) {
	one := sampleFirmwareInstance()
	two := sampleFirmwareInstance()
	two.Name = "other"
	two.Address = [config.AddrLen]byte{0x99}

	got, err := resolveFirmwareInstance(inventory.Inventory{one}, "")
	if err != nil {
		t.Fatalf("sole instance: %v", err)
	}
	if got.Name != one.Name {
		t.Fatalf("sole instance = %q, want %q", got.Name, one.Name)
	}

	if _, err := resolveFirmwareInstance(inventory.Inventory{one, two}, ""); err == nil || !strings.Contains(err.Error(), "name one") {
		t.Fatalf("ambiguous instance error = %v", err)
	}
	if got, err := resolveFirmwareInstance(inventory.Inventory{one, two}, "other"); err != nil || got.Name != "other" {
		t.Fatalf("named instance = %q, %v", got.Name, err)
	}
	if _, err := resolveFirmwareInstance(inventory.Inventory{one}, "missing"); err == nil || !strings.Contains(err.Error(), "no inventory instance") {
		t.Fatalf("unknown instance error = %v", err)
	}

	legacy := sampleInstance()
	if _, err := resolveFirmwareInstance(inventory.Inventory{legacy}, ""); err == nil || !strings.Contains(err.Error(), "no importable firmware manifest") {
		t.Fatalf("legacy instance error = %v", err)
	}
}

func sampleFirmwareInstance() inventory.Instance {
	inst := sampleInstance()
	inst.Type.Firmware = firmware.Manifest{
		Package: "example.com/acme/board/fw",
		TinyGo: firmware.TinyGoProfile{
			Scheduler: firmware.SchedulerTasks,
		},
	}
	return inst
}

func findChild(parent interface{ Commands() []*cobra.Command }, name string) *cobra.Command {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	return nil
}
