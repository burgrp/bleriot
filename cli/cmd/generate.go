// generate.go implements the "bleriot generate" subcommand: the host-side code
// generator that turns a hand-authored register spec into the two artifacts
// BleRiot needs from a single run (see the protocol spec, §11.7):
//
//   - <out>/<node>_gen.go   firmware node code (const wire IDs + RegisterIDs + version)
//   - <out>/<node>.json      hub node descriptor (resolved id → name/type/scaling)
//
// Because both come from one run, firmware and hub can never drift. The spec is
// a JSON file (--spec) mirroring the authoring model: a class library plus a
// node composed of class instances. Wire IDs are never authored; they are
// assigned deterministically by the generator.
//
// Example spec:
//
//	{
//	  "node": "garage-controller",
//	  "metadata": { "hw_rev": "1.3" },
//	  "classes": [
//	    {
//	      "name": "thermometer",
//	      "metadata": { "category": "sensor" },
//	      "registers": [
//	        { "name": "temperature", "type": "float", "multiplier": 1, "divider": 100, "metadata": { "unit": "celsius" } },
//	        { "name": "humidity", "type": "int" }
//	      ]
//	    },
//	    { "name": "switch", "registers": [ { "name": "relay", "type": "bool" } ] }
//	  ],
//	  "instances": [
//	    { "class": "thermometer", "name": "outdoor" },
//	    { "class": "switch", "name": "main" }
//	  ]
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
// wire IDs (IDs are assigned by descriptor.AllocateIDs).
type specFile struct {
	Node      string            `json:"node"`
	Metadata  map[string]string `json:"metadata"`
	Classes   []specClass       `json:"classes"`
	Instances []specInstance    `json:"instances"`
}

type specClass struct {
	Name      string            `json:"name"`
	Metadata  map[string]string `json:"metadata"`
	Registers []specRegister    `json:"registers"`
}

type specRegister struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Multiplier int32             `json:"multiplier"`
	Divider    int32             `json:"divider"`
	Metadata   map[string]string `json:"metadata"`
}

type specInstance struct {
	Class string `json:"class"`
	Name  string `json:"name"`
}

// newGenerateCmd builds the "generate" subcommand.
func newGenerateCmd() *cobra.Command {
	var (
		specPath string
		outDir   string
		pkg      string
	)
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate firmware node code and the hub descriptor from a spec",
		Long: "Generate the two BleRiot artifacts from a single hand-authored JSON " +
			"spec: the firmware node code (const wire IDs + version) and the hub " +
			"node descriptor (resolved id → name/type/scaling). Wire IDs are " +
			"assigned deterministically so firmware and hub can never drift.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerate(specPath, outDir, pkg, slog.Default())
		},
	}
	cmd.Flags().StringVar(&specPath, "spec", "node.json", "path to the JSON node spec")
	cmd.Flags().StringVar(&outDir, "out", ".", "directory to write the generated artifacts into")
	cmd.Flags().StringVar(&pkg, "package", "main", "Go package name for the generated firmware code")
	return cmd
}

func runGenerate(specPath, outDir, pkg string, logger *slog.Logger) error {
	spec, err := loadSpec(specPath)
	if err != nil {
		return err
	}

	library := make(map[string]descriptor.ClassDescriptor, len(spec.Classes))
	for _, c := range spec.Classes {
		if _, dup := library[c.Name]; dup {
			return fmt.Errorf("duplicate class %q", c.Name)
		}
		regs := make([]descriptor.RegisterDescriptor, 0, len(c.Registers))
		for _, r := range c.Registers {
			regs = append(regs, descriptor.RegisterDescriptor{
				Name:       r.Name,
				Type:       descriptor.RegType(r.Type),
				Multiplier: r.Multiplier,
				Divider:    r.Divider,
				Metadata:   r.Metadata,
			})
		}
		library[c.Name] = descriptor.ClassDescriptor{
			Name:      c.Name,
			Registers: regs,
			Metadata:  c.Metadata,
		}
	}

	nodeSpec := descriptor.NodeSpec{
		Name:      spec.Node,
		Metadata:  spec.Metadata,
		Instances: make([]descriptor.ClassInstance, 0, len(spec.Instances)),
	}
	for _, i := range spec.Instances {
		nodeSpec.Instances = append(nodeSpec.Instances, descriptor.ClassInstance{
			Class: i.Class,
			Name:  i.Name,
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

	logger.Info("generated node artifacts",
		"node", res.Node,
		"registers", len(res.Registers),
		"version", fmt.Sprintf("0x%08X", res.Version),
		"code", codePath,
		"descriptor", jsonPath,
	)
	return nil
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
