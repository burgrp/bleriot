package cli

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/burgrp/bleriot/lib/shared/config"
	"github.com/burgrp/bleriot/lib/shared/inventory"
)

// newNewCmd builds the "new" subcommand: generate a random address and key and
// print a paste-ready inventory.Instance stub. It is entirely offline and does
// not require an attached device or debug probe.
func newNewCmd(inv inventory.Inventory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Print an inventory Instance stub with a new random identity",
		Long: "Generate a random nonzero RF address and XTEA key, then print a " +
			"paste-ready inventory.Instance ready to drop into the inventory source. " +
			"No device or debug probe is required.",
		Args: cobra.NoArgs,
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runNew(inv, rand.Reader, os.Stdout)
	}
	return cmd
}

func runNew(inv inventory.Inventory, random io.Reader, w io.Writer) error {
	address, err := randomAddress(inv, random)
	if err != nil {
		return err
	}

	var key [config.KeyLen]byte
	if _, err := io.ReadFull(random, key[:]); err != nil {
		return fmt.Errorf("generating key: %w", err)
	}

	_, err = fmt.Fprint(w, instanceStub(address, key))
	return err
}

// randomAddress generates a nonzero address not already present in inv.
func randomAddress(inv inventory.Inventory, random io.Reader) ([config.AddrLen]byte, error) {
	for {
		var address [config.AddrLen]byte
		if _, err := io.ReadFull(random, address[:]); err != nil {
			return address, fmt.Errorf("generating address: %w", err)
		}
		if address == ([config.AddrLen]byte{}) || hasAddress(inv, address) {
			continue
		}
		return address, nil
	}
}

// instanceStub renders a paste-ready inventory.Instance literal with the given
// random address and key filled in and the rest left as TODO placeholders.
func instanceStub(address [config.AddrLen]byte, key [config.KeyLen]byte) string {
	return fmt.Sprintf(`{
	Name:    "TODO",
	Address: %s,
	Key:     %s,
	Channel: inventory.Channel{Number: 0}, // TODO: e.g. a shared channel var
	Type:    TODO, // device type, e.g. spec.Type()
	Config:  nil,  // TODO: device config, e.g. spec.Config{...}
},
`, byteArrayLiteral(address[:]), byteArrayLiteral(key[:]))
}

// byteArrayLiteral renders bytes as a Go fixed-array literal.
func byteArrayLiteral(b []byte) string {
	out := fmt.Sprintf("[%d]byte{", len(b))
	for i, v := range b {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("0x%02X", v)
	}
	out += "}"
	return out
}

func hasAddress(inv inventory.Inventory, address [config.AddrLen]byte) bool {
	for _, inst := range inv {
		if inst.Address == address {
			return true
		}
	}
	return false
}
