package cli

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/burgrp/bleriot/shared/config"
	"github.com/burgrp/bleriot/shared/inventory"
)

// newNewCmd builds the "new" subcommand: read an attached, not-yet-known
// device's UID over SWD and print a paste-ready inventory.Instance stub for it.
// This is how a device is onboarded into the inventory source.
//
// The chip to read over SWD comes from the device types' declared Chip; --chip
// selects one when the inventory declares more than one (or onboards the very
// first device of an empty inventory by built-in chip name).
func newNewCmd(inv inventory.Inventory) *cobra.Command {
	var chipName string
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Print an inventory Instance stub for the attached device",
		Long: "Read the attached device's MCU unique ID over SWD and print a " +
			"paste-ready inventory.Instance for it, with a freshly generated random " +
			"XTEA key, ready to drop into the inventory source. Warns if the UID " +
			"already belongs to an inventory instance.",
		Args: cobra.NoArgs,
	}
	cmd.Flags().StringVar(&chipName, "chip", "", "chip to read over SWD (required only if the inventory declares more than one)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		chip, err := resolveChip(inv, chipName)
		if err != nil {
			return err
		}
		return runNew(cmd.Context(), inv, chipProbe(chip, slog.Default()), os.Stdout, slog.Default())
	}
	return cmd
}

func runNew(ctx context.Context, inv inventory.Inventory, probe Probe, w io.Writer, logger *slog.Logger) error {
	uid, err := probe.ReadUID(ctx)
	if err != nil {
		return err
	}
	if inst, ok := findByUID(inv, uid); ok {
		logger.Warn("device already in inventory", "name", inst.Name, "uid", fmt.Sprintf("%X", uid))
	}

	// Each device needs a unique secret key (§5); generate it here so it is never
	// hand-invented. The operator commits the printed stub, key and all.
	var key [config.KeyLen]byte
	if _, err := rand.Read(key[:]); err != nil {
		return fmt.Errorf("generating key: %w", err)
	}

	_, err = fmt.Fprint(w, instanceStub(uid, key))
	return err
}

// instanceStub renders a paste-ready inventory.Instance literal with the given
// UID and freshly generated key filled in and the rest left as TODO placeholders.
func instanceStub(uid [config.UIDLen]byte, key [config.KeyLen]byte) string {
	return fmt.Sprintf(`{
	Name:    "TODO",
	UID:     %s,
	Key:     %s,
	Channel: inventory.Channel{Number: 0}, // TODO: e.g. a shared channel var
	Type:    TODO, // device type, e.g. spec.Type()
	Config:  nil,  // TODO: device config, e.g. spec.Config{...}
},
`, byteArrayLiteral(uid[:]), byteArrayLiteral(key[:]))
}

// byteArrayLiteral renders bytes as a Go fixed-array literal, e.g.
// [12]byte{0x01, 0x02, ...}.
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
