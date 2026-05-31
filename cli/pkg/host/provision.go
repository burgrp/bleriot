package host

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"cli/pkg/inventory"
	"cli/pkg/node"
	"cli/pkg/page"
)

// probeConfig holds the flags that configure a hardware Probe (the pyocd target
// and the MCU addresses it reads/writes). It is shared by the provision and new
// commands.
type probeConfig struct {
	target   string
	uidAddr  uint32
	pageAddr uint32
}

// addProbeFlags registers the probe flags on cmd and returns the config they
// populate.
func addProbeFlags(cmd *cobra.Command) *probeConfig {
	pc := &probeConfig{}
	f := cmd.Flags()
	f.StringVar(&pc.target, "target", "py32f030x8", "pyocd target name")
	f.Uint32Var(&pc.uidAddr, "uid-addr", 0x1FFF0E00, "memory address of the 12-byte MCU unique ID")
	f.Uint32Var(&pc.pageAddr, "page-addr", 0x0800F800, "flash address of the provisioning page")
	return pc
}

// probe builds the hardware Probe from the flag values.
func (pc *probeConfig) probe(logger *slog.Logger) Probe {
	return &PyOCDProbe{
		Target:   pc.target,
		UIDAddr:  pc.uidAddr,
		PageAddr: pc.pageAddr,
		Logger:   logger,
	}
}

// newProvisionCmd builds the "provision" subcommand: read the attached device's
// UID over SWD, find the matching inventory instance, and write its provisioning
// page (identity + config) to flash. The device is matched by UID alone, so no
// device name argument is needed.
func newProvisionCmd(inv inventory.Inventory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision the attached device from its inventory entry",
		Long: "Read the attached device's MCU unique ID over SWD, look it up in the " +
			"inventory, and write its provisioning page (RF address, key, channel and " +
			"config) to flash.",
		Args: cobra.NoArgs,
	}
	pc := addProbeFlags(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runProvision(cmd.Context(), inv, pc.probe(slog.Default()), slog.Default())
	}
	return cmd
}

func runProvision(ctx context.Context, inv inventory.Inventory, probe Probe, logger *slog.Logger) error {
	if err := inv.Validate(); err != nil {
		return fmt.Errorf("inventory: %w", err)
	}
	uid, err := probe.ReadUID(ctx)
	if err != nil {
		return err
	}
	inst, ok := findByUID(inv, uid)
	if !ok {
		return fmt.Errorf("no inventory instance has UID %X; run \"new\" to add one", uid)
	}

	addr := node.AddressFromUID(inst.UID)
	image, err := page.Marshal(addr, inst.Key, inst.Channel, inst.Config)
	if err != nil {
		return fmt.Errorf("instance %q: building page: %w", inst.Name, err)
	}
	if err := probe.WritePage(ctx, image); err != nil {
		return fmt.Errorf("instance %q: writing page: %w", inst.Name, err)
	}

	logger.Info("provisioned device", "name", inst.Name, "address", fmt.Sprintf("%X", addr), "channel", inst.Channel, "bytes", len(image))
	return nil
}

// findByUID returns the instance whose UID matches uid.
func findByUID(inv inventory.Inventory, uid [page.UIDLen]byte) (inventory.Instance, bool) {
	for _, inst := range inv {
		if inst.UID == uid {
			return inst, true
		}
	}
	return inventory.Instance{}, false
}
