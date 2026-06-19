package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"site/config"
	"site/inventory"
	"site/node"
)

// newProvisionCmd builds the "provision" subcommand: read the attached device's
// UID over SWD, find the matching inventory instance, and write its provisioning
// page (identity + config) to flash. The device is matched by UID alone, so no
// device name argument is needed.
//
// The chip to drive over SWD comes from the device types' declared Chip; --chip
// selects one when the inventory declares more than one.
func newProvisionCmd(inv inventory.Inventory) *cobra.Command {
	var chipName string
	cmd := &cobra.Command{
		Use:   "provision",
		Short: "Provision the attached device from its inventory entry",
		Long: "Read the attached device's MCU unique ID over SWD, look it up in the " +
			"inventory, and write its provisioning page (RF address, key, channel and " +
			"config) to flash.",
		Args: cobra.NoArgs,
	}
	cmd.Flags().StringVar(&chipName, "chip", "", "chip to drive over SWD (required only if the inventory declares more than one)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		chip, err := resolveChip(inv, chipName)
		if err != nil {
			return err
		}
		return runProvision(cmd.Context(), inv, chip, chipProbe(chip, slog.Default()), slog.Default())
	}
	return cmd
}

func runProvision(ctx context.Context, inv inventory.Inventory, chip inventory.Chip, probe Probe, logger *slog.Logger) error {
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

	// Guard against driving the wrong chip: the matched device's type must run on
	// the chip we are about to write to, or we'd flash to the wrong page address.
	if got := inst.Type.Chip; got.Name != "" && chip.Name != "" && got != chip {
		return fmt.Errorf("attached device %q runs on chip %q, but %q was selected; pass --chip %s",
			inst.Name, got.Name, chip.Name, got.Name)
	}

	addr := node.AddressFromUID(inst.UID)
	image, err := config.Marshal(addr, inst.Key, inst.Channel.Number, inst.Channel.SpreadFactor, inst.Config)
	if err != nil {
		return fmt.Errorf("instance %q: building page: %w", inst.Name, err)
	}
	if err := probe.WritePage(ctx, image); err != nil {
		return fmt.Errorf("instance %q: writing page: %w", inst.Name, err)
	}

	logger.Info("provisioned device", "name", inst.Name, "address", fmt.Sprintf("%X", addr), "channel", inst.Channel.Number, "spreadFactor", inst.Channel.SpreadFactor, "bytes", len(image))
	return nil
}

// findByUID returns the instance whose UID matches uid.
func findByUID(inv inventory.Inventory, uid [config.UIDLen]byte) (inventory.Instance, bool) {
	for _, inst := range inv {
		if inst.UID == uid {
			return inst, true
		}
	}
	return inventory.Instance{}, false
}
