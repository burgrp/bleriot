package host

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"cli/pkg/inventory"
	"cli/pkg/page"
)

// newNewCmd builds the "new" subcommand: read an attached, not-yet-known
// device's UID over SWD and print a paste-ready inventory.Instance stub for it.
// This is how a device is onboarded into the inventory source.
func newNewCmd(inv inventory.Inventory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Print an inventory Instance stub for the attached device",
		Long: "Read the attached device's MCU unique ID over SWD and print a " +
			"paste-ready inventory.Instance for it, ready to drop into the inventory " +
			"source. Warns if the UID already belongs to an inventory instance.",
		Args: cobra.NoArgs,
	}
	pc := addProbeFlags(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runNew(cmd.Context(), inv, pc.probe(slog.Default()), os.Stdout, slog.Default())
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
	_, err = fmt.Fprint(w, instanceStub(uid))
	return err
}

// instanceStub renders a paste-ready inventory.Instance literal with the given
// UID filled in and the rest left as TODO placeholders.
func instanceStub(uid [page.UIDLen]byte) string {
	return fmt.Sprintf(`inventory.Instance{
	Name:    "TODO",
	UID:     %s,
	Key:     [16]byte{ /* TODO: 16-byte XTEA key */ },
	Channel: 0, // TODO
	Type:    TODO, // device type, e.g. thermostat.Type()
	Config:  nil,  // TODO: device config, e.g. thermostat.Config{...}
},
`, byteArrayLiteral(uid[:]))
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
