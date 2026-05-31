// generate.go implements the "bleriot generate" subcommand: the host-side code
// generator that turns a hand-authored register spec into the two artifacts
// BleRiot needs from a single run (see the protocol spec, §11.7):
//
//		bleriot generate <spec> <fw-go>
//
//	  - <fw-go>        firmware node code (const wire IDs + RegisterIDs + descriptor ID)
//	  - <id>.json      hub node descriptor (resolved id → name/type/scaling),
//	                   written next to <spec> and named by its descriptor ID
//
// The descriptor file is content-addressed: its name is the descriptor ID, a
// hash over the resolved register set. The same ID is embedded
// in the firmware as DescriptorID, so a provisioned device can report which
// descriptor it implements and the hub can select it from a pool (§11.9).
//
// Because both artifacts come from one run, firmware and hub can never drift. The
// spec is a JSON file mirroring the authoring model: a class library plus a node
// composed of class instances. Wire IDs are never authored; they are assigned
// deterministically by the generator.
//
// Example spec:
//
//	{
//	  "node": "garage-controller",
//	  "metadata": { "hw_rev": "1.3" },
//	  "classes": {
//	    "thermometer": {
//	      "metadata": { "category": "sensor" },
//	      "registers": {
//	        "temperature": { "type": "float", "multiplier": 1, "divider": 100, "metadata": { "unit": "celsius" } },
//	        "humidity": { "type": "int" }
//	      }
//	    },
//	    "switch": { "registers": { "relay": { "type": "bool" } } }
//	  },
//	  "instances": {
//	    "outdoor": "thermometer",
//	    "main": "switch"
//	  }
//	}
package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"cli/pkg/codegen"
	"cli/pkg/descriptor"
)

// specFile is the JSON authoring format consumed by "bleriot generate". It
// mirrors descriptor.NodeSpec + the class library, but carries JSON tags and no
// wire IDs (IDs are assigned by descriptor.AllocateIDs). Classes, registers, and
// instances are keyed by name: the map key is the name, never a field. An
// instance maps its name to the class it instantiates.
type specFile struct {
	Node      string               `json:"node"`
	Metadata  map[string]string    `json:"metadata"`
	Classes   map[string]specClass `json:"classes"`
	Instances map[string]string    `json:"instances"`
}

type specClass struct {
	Metadata  map[string]string       `json:"metadata"`
	Registers map[string]specRegister `json:"registers"`
}

type specRegister struct {
	Type       string            `json:"type"`
	Multiplier int32             `json:"multiplier"`
	Divider    int32             `json:"divider"`
	Metadata   map[string]string `json:"metadata"`
}

// newGenerateCmd builds the "generate" subcommand.
func newGenerateCmd() *cobra.Command {
	var pkg string
	cmd := &cobra.Command{
		Use:   "generate <spec> <fw-go>",
		Short: "Generate firmware node code and the hub descriptor from a spec",
		Long: "Generate the two BleRiot artifacts from a single hand-authored JSON " +
			"spec: the firmware node code (const wire IDs + descriptor ID) and the hub " +
			"node descriptor (resolved id → name/type/scaling). Wire IDs are " +
			"assigned deterministically so firmware and hub can never drift.\n\n" +
			"The descriptor is content-addressed: it is written next to <spec> and " +
			"named <id>.json, where <id> is the descriptor ID (also " +
			"embedded in the firmware).\n\n" +
			"Arguments:\n" +
			"  spec   path to the JSON node spec\n" +
			"  fw-go  output path for the generated firmware Go source",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerate(args[0], args[1], pkg, slog.Default())
		},
	}
	cmd.Flags().StringVar(&pkg, "package", "main", "Go package name for the generated firmware code")
	return cmd
}

func runGenerate(specPath, fwGoPath, pkg string, logger *slog.Logger) error {
	spec, err := loadSpec(specPath)
	if err != nil {
		return err
	}

	library := make(map[string]descriptor.ClassDescriptor, len(spec.Classes))
	for className, c := range spec.Classes {
		regs := make([]descriptor.RegisterDescriptor, 0, len(c.Registers))
		for regName, r := range c.Registers {
			regs = append(regs, descriptor.RegisterDescriptor{
				Name:       regName,
				Type:       descriptor.RegType(r.Type),
				Multiplier: r.Multiplier,
				Divider:    r.Divider,
				Metadata:   r.Metadata,
			})
		}
		library[className] = descriptor.ClassDescriptor{
			Name:      className,
			Registers: regs,
			Metadata:  c.Metadata,
		}
	}

	nodeSpec := descriptor.NodeSpec{
		Name:      spec.Node,
		Metadata:  spec.Metadata,
		Instances: make([]descriptor.ClassInstance, 0, len(spec.Instances)),
	}
	for instName, className := range spec.Instances {
		nodeSpec.Instances = append(nodeSpec.Instances, descriptor.ClassInstance{
			Class: className,
			Name:  instName,
		})
	}

	res, err := descriptor.AllocateIDs(nodeSpec, library)
	if err != nil {
		return fmt.Errorf("allocate IDs: %w", err)
	}

	nodeCode, err := codegen.GenerateNodeCode(res, codegen.NodeCodeOptions{Package: pkg})
	if err != nil {
		return err
	}
	descJSON, err := codegen.GenerateDescriptorJSON(res)
	if err != nil {
		return err
	}

	// The descriptor is content-addressed: its file name is the descriptor ID
	// (also embedded in the firmware), written next to the spec.
	descID := fmt.Sprintf("%08X", res.Version)
	nodeJSONPath := filepath.Join(filepath.Dir(specPath), descID+".json")

	if err := writeFile(fwGoPath, nodeCode); err != nil {
		return err
	}
	if err := writeFile(nodeJSONPath, descJSON); err != nil {
		return err
	}

	logger.Info("generated node artifacts",
		"node", res.Node,
		"registers", len(res.Registers),
		"descriptorId", descID,
		"code", fwGoPath,
		"descriptor", nodeJSONPath,
	)
	return nil
}

// writeFile writes data to path, creating any missing parent directories.
func writeFile(path string, data []byte) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}

// loadSpec reads and parses the JSON node spec.
func loadSpec(path string) (specFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return specFile{}, fmt.Errorf("reading spec %s: %w", path, err)
	}
	var spec specFile
	if err := json.Unmarshal(data, &spec); err != nil {
		return specFile{}, fmt.Errorf("parsing spec %s: %w", path, err)
	}
	if spec.Node == "" {
		return specFile{}, fmt.Errorf("spec %s: node is required", path)
	}
	return spec, nil
}
