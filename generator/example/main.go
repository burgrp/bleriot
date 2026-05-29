// Command example demonstrates the BleRiot generator end to end: it defines a
// small class library and a node spec in Go (the authoring format), resolves
// wire IDs, and writes the two generated artifacts (PROTOCOL.md §11.7):
//
//	<out>/<node>_gen.go        firmware node code (const IDs + RegisterIDs + version)
//	<out>/<node>.json          hub node descriptor
//
// Run for manual testing:
//
//	go run ./example            # writes to ./out
//	go run ./example /tmp/bgen  # writes to /tmp/bgen
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"generator/codegen"
	"generator/descriptor"
)

func main() {
	outDir := "out"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	if err := run(outDir); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(outDir string) error {
	library := map[string]descriptor.ClassDescriptor{
		"thermometer": {
			Name:     "thermometer",
			Metadata: map[string]string{"category": "sensor"},
			Registers: []descriptor.RegisterDescriptor{
				{Name: "temperature", Type: descriptor.TypeFloat, Multiplier: 1, Divider: 100, Metadata: map[string]string{"unit": "celsius"}},
				{Name: "humidity", Type: descriptor.TypeInt, Multiplier: 1, Divider: 1, Metadata: map[string]string{"unit": "percent"}},
			},
		},
		"switch": {
			Name: "switch",
			Registers: []descriptor.RegisterDescriptor{
				{Name: "relay", Type: descriptor.TypeBool},
			},
		},
	}

	spec := descriptor.NodeSpec{
		Name:     "garage-controller",
		Metadata: map[string]string{"hw_rev": "1.3"},
		Instances: []descriptor.ClassInstance{
			{Class: "thermometer", Name: "outdoor"},
			{Class: "thermometer", Name: "indoor"},
			{Class: "switch", Name: "main"},
			{Class: "switch", Name: "aux"},
		},
	}

	res, err := descriptor.AllocateIDs(spec, library)
	if err != nil {
		return fmt.Errorf("allocate IDs: %w", err)
	}

	nodeCode, err := codegen.GenerateNodeCode(res, codegen.NodeCodeOptions{Package: "main"})
	if err != nil {
		return err
	}
	descJSON, err := codegen.GenerateDescriptorJSON(res)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	codePath := filepath.Join(outDir, res.Node+"_gen.go")
	jsonPath := filepath.Join(outDir, res.Node+".json")
	if err := os.WriteFile(codePath, nodeCode, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, descJSON, 0o644); err != nil {
		return err
	}

	fmt.Printf("node %q: %d registers (version 0x%08X)\n", res.Node, len(res.Registers), res.Version)
	fmt.Printf("  node code: %s\n", codePath)
	fmt.Printf("  descriptor: %s\n", jsonPath)
	return nil
}
