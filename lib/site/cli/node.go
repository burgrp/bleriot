package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/burgrp/bleriot/lib/shared/inventory"
	"github.com/burgrp/bleriot/lib/site/fwbuild"
)

// newNodeCmd groups firmware, debugging, and identity operations for inventory
// nodes. Each command that selects an existing node owns its --name flag; new
// creates an identity and therefore does not select one.
func newNodeCmd(inv inventory.Inventory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Build, debug, and provision inventory nodes",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newNodeBuildCmd(inv))
	cmd.AddCommand(newNodeGenCmd(inv))
	cmd.AddCommand(newNodeNewCmd(inv))
	cmd.AddCommand(newNodeToolCmd(inv, "rtt", "Open an RTT session without rebuilding", fwbuild.RTT))
	cmd.AddCommand(newNodeToolCmd(inv, "gdb", "Start a pyOCD GDB server without rebuilding", fwbuild.GDB))
	cmd.AddCommand(newNodeToolCmd(inv, "install-pack", "Install the node chip's pyOCD CMSIS pack", fwbuild.InstallPack))
	return cmd
}

func newNodeBuildCmd(inv inventory.Inventory) *cobra.Command {
	var name string
	var options fwbuild.BuildOptions
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build firmware and optionally disassemble, flash, or open RTT",
		Long: "Build the selected inventory node's firmware with its identity and config baked in. " +
			"Optional stages always run in the order disassembly, flash, then RTT.",
		Args: cobra.NoArgs,
	}
	addNodeNameFlag(cmd, &name)
	cmd.Flags().BoolVar(&options.Disassembly, "disassembly", false, "write disassembly.txt after building")
	cmd.Flags().BoolVar(&options.Flash, "flash", false, "flash the built image")
	cmd.Flags().BoolVar(&options.RTT, "rtt", false, "open RTT after the requested build stages")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		inst, err := resolveFirmwareInstance(inv, name)
		if err != nil {
			return err
		}
		source, err := renderProvisioning(inst)
		if err != nil {
			return fmt.Errorf("instance %q: %w", inst.Name, err)
		}
		request := firmwareRequest(inst)
		request.Source = source
		return fwbuild.Build(cmd.Context(), request, options)
	}
	return cmd
}

type nodeTool func(context.Context, fwbuild.Request) error

func newNodeToolCmd(inv inventory.Inventory, use, short string, run nodeTool) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
	}
	addNodeNameFlag(cmd, &name)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		inst, err := resolveFirmwareInstance(inv, name)
		if err != nil {
			return err
		}
		return run(cmd.Context(), firmwareRequest(inst))
	}
	return cmd
}

func addNodeNameFlag(cmd *cobra.Command, name *string) {
	cmd.Flags().StringVar(name, "name", "", "inventory node name (required when inventory contains multiple nodes)")
}

func resolveFirmwareInstance(inv inventory.Inventory, name string) (inventory.Instance, error) {
	if err := inv.Validate(); err != nil {
		return inventory.Instance{}, fmt.Errorf("inventory: %w", err)
	}
	inst, err := resolveInstance(inv, name)
	if err != nil {
		return inventory.Instance{}, err
	}
	if inst.Type.Firmware.IsZero() {
		return inventory.Instance{}, fmt.Errorf("node %q device type %q has no importable firmware manifest", inst.Name, inst.Type.Name)
	}
	return inst, nil
}

func firmwareRequest(inst inventory.Instance) fwbuild.Request {
	return fwbuild.Request{
		InstanceName: inst.Name,
		Manifest:     inst.Type.Firmware,
		Chip:         inst.Type.Chip,
		Stdin:        os.Stdin,
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
	}
}
