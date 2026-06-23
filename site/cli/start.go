// Package cli is the BleRiot host runtime that a site repository drives with
// its inventory-as-code. A site's main() declares its deployment as an
// inventory.Inventory and hands it to Start:
//
//	func main() {
//		cli.Start(inventory.Inventory{ ... })
//	}
//
// Start builds the "bleriot" command tree around that inventory and runs it. The
// inventory is the single source of truth for which devices exist, their
// identities, types and configuration; runtime/deploy concerns (registry URL,
// hub RF address, serial ports, timeouts) are command-line flags, not inventory
// data.
//
// Subcommands:
//
//	hub        bridge the inventory's nodes to the Registry
//	provision  write a device's identity + config to its flash over SWD
//	new        read an attached device's UID and print an Instance stub
package cli

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/spf13/cobra"

	"github.com/burgrp/bleriot/shared/inventory"
	"github.com/burgrp/bleriot/site/node"
)

// debug is the global verbosity flag, applied before any subcommand runs.
var debug bool

// Start builds and runs the host command tree for the given inventory. It is the
// single entry point a site binary calls from main(). Start never returns on
// error; it prints a plain message and exits non-zero, matching a CLI's
// behaviour.
func Start(inv inventory.Inventory) {
	if err := newRootCmd(inv).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// newRootCmd builds the top-level "bleriot" command around inv and wires its
// subcommands.
func newRootCmd(inv inventory.Inventory) *cobra.Command {
	root := &cobra.Command{
		Use:   "bleriot",
		Short: "BleRiot host for an inventory-as-code deployment",
		Long: "bleriot runs the host side of a BleRiot deployment described in code: " +
			"bridge nodes to the Registry, provision devices, and onboard new ones.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			level := slog.LevelInfo
			if debug {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(tint.NewHandler(os.Stderr, &tint.Options{
				Level:      level,
				TimeFormat: time.TimeOnly,
			})))
		},
	}
	root.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging (shows serial communication)")
	root.AddCommand(newHubCmd(inv))
	root.AddCommand(newProvisionCmd(inv))
	root.AddCommand(newNewCmd(inv))
	return root
}

// buildNode converts an inventory Instance into the engine/bridge view of a
// node: its register descriptor plus its provisioned identity. The RF address is
// derived from the device's UID (it is never stored in the inventory).
func buildNode(inst inventory.Instance) (*node.Node, error) {
	desc, err := descriptorFor(inst.Type)
	if err != nil {
		return nil, err
	}
	id := node.Identity{
		Address: node.AddressFromUID(inst.UID),
		Key:     inst.Key,
	}
	return node.NewNode(inst.Name, inst.Channel.Number, desc, id), nil
}

// descriptorFor builds a node.Descriptor from a device type's register table.
// Each register's permanent Tag becomes its wire ID (the engine keeps registers
// as uint16); slice order is irrelevant.
func descriptorFor(dt inventory.DeviceType) (*node.Descriptor, error) {
	regs := make([]node.Register, len(dt.Registers))
	for i, r := range dt.Registers {
		regs[i] = node.Register{
			ID:         uint16(r.Tag),
			Name:       r.Name,
			Type:       node.RegType(r.Type),
			Multiplier: r.Multiplier,
			Divider:    r.Divider,
			Metadata:   r.Metadata,
		}
	}
	return node.NewDescriptor(map[string]string{"device": dt.Name}, regs)
}
