package cli

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/spf13/cobra"

	"github.com/burgrp/bleriot/lib/shared/inventory"
)

// newMakeCmd builds the "make" subcommand: a thin wrapper around GNU make that
// builds or flashes one inventory instance's firmware straight from the hub. It
// locates the device type's firmware source tree, then runs make there with two
// things injected from the authoritative hub inventory:
//
//   - GEN, the command the Makefile runs to produce the baked-in provisioning
//     source. It points back at this binary's own "gen <name>" subcommand, so the
//     identity and config come from the hub, not from whatever inventory (if any)
//     lives next to the firmware.
//   - the chip's build/flash targets (TARGET_TINYGO, TARGET_PYOCD, CMSIS_PACK) as
//     make variables, so the Makefile need not hard-code them.
//
// Everything after the instance name is passed straight through to make, e.g.
// "bleriot make bob flash".
func newMakeCmd(inv inventory.Inventory) *cobra.Command {
	var root string
	cmd := &cobra.Command{
		Use:   "make <name> [make-args...]",
		Short: "Build/flash a device's firmware via GNU make, with its identity injected",
		Long: "Run GNU make on a device's firmware source tree with the named inventory " +
			"instance's identity and config injected (as the provisioning generator) and its chip's " +
			"build/flash targets injected as make variables. The firmware source tree is inferred " +
			"from the device type's Config package (the nearest enclosing directory with a Makefile), " +
			"or given with --root. Arguments after the name pass straight through to make, e.g. " +
			"\"bleriot make bob flash\". Requires the firmware source tree and toolchain (make, tinygo, " +
			"pyocd) to be present, so run it from a checkout of the hub, not a stripped binary.",
		Args: cobra.MinimumNArgs(1),
	}
	// Stop flag parsing at the first positional so make's own flags (e.g. -j) pass
	// through untouched; --root must therefore precede the instance name.
	cmd.Flags().SetInterspersed(false)
	cmd.Flags().StringVar(&root, "root", "", "firmware source tree (default: inferred from the device type's Config package)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runMake(inv, args[0], root, args[1:], os.Stdout, os.Stderr)
	}
	return cmd
}

// runMake resolves the instance, locates its firmware source tree, and runs make
// there with the hub's identity generator and the chip's targets injected. make's
// stdio is wired straight through so its output (and any RTT console it launches)
// reaches the operator.
func runMake(inv inventory.Inventory, name, root string, userArgs []string, stdout, stderr io.Writer) error {
	if err := inv.Validate(); err != nil {
		return fmt.Errorf("inventory: %w", err)
	}
	inst, err := resolveInstance(inv, name)
	if err != nil {
		return err
	}
	if root == "" {
		root, err = inferRoot(inst)
		if err != nil {
			return err
		}
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating bleriot executable: %w", err)
	}
	genCmd := fmt.Sprintf("%s gen %s", self, inst.Name)
	args := buildMakeArgs(root, genCmd, inst.Type.Chip, userArgs)

	slog.Debug("running make", "dir", root, "args", args)
	c := exec.Command("make", args...)
	c.Stdout = stdout
	c.Stderr = stderr
	c.Stdin = os.Stdin
	return c.Run()
}

// buildMakeArgs assembles make's argv: "-C <root>", the user's passthrough
// arguments, then the injected variable assignments. GEN overrides the Makefile's
// default generator so the provisioning source is baked from the hub inventory;
// the TARGET_/CMSIS_ vars supply the chip's build and flash targets. Empty chip
// fields are omitted so the Makefile keeps its own defaults for them.
func buildMakeArgs(root, genCmd string, chip inventory.Chip, userArgs []string) []string {
	args := append([]string{"-C", root}, userArgs...)
	args = append(args, "GEN="+genCmd)
	if chip.TinygoTarget != "" {
		args = append(args, "TARGET_TINYGO="+chip.TinygoTarget)
	}
	if chip.PyocdTarget != "" {
		args = append(args, "TARGET_PYOCD="+chip.PyocdTarget)
	}
	if chip.CmsisPack != "" {
		args = append(args, "CMSIS_PACK="+chip.CmsisPack)
	}
	return args
}

// inferRoot locates the firmware source tree for an instance from its Config
// type: it resolves the type's package directory with "go list", then walks up to
// the nearest directory containing a Makefile. A nil Config or an unnamed type
// leaves nothing to locate, so the caller must pass --root instead.
func inferRoot(inst inventory.Instance) (string, error) {
	if inst.Config == nil {
		return "", fmt.Errorf("instance %q has no Config to locate the firmware source; pass --root", inst.Name)
	}
	t := reflect.TypeOf(inst.Config)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	pkgPath := t.PkgPath()
	if pkgPath == "" {
		return "", fmt.Errorf("instance %q Config type %s has no import path; pass --root", inst.Name, t)
	}
	dir, err := goListDir(pkgPath)
	if err != nil {
		return "", fmt.Errorf("locating package %q: %w", pkgPath, err)
	}
	root, err := findMakefileRoot(dir)
	if err != nil {
		return "", fmt.Errorf("%w; pass --root", err)
	}
	return root, nil
}

// goListDir returns the on-disk directory of a Go import path via "go list". The
// package is reachable because the hub inventory references its Config type, so
// running from the hub's module resolves it.
func goListDir(pkgPath string) (string, error) {
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", pkgPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("go list: %s", msg)
		}
		return "", fmt.Errorf("go list: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// findMakefileRoot walks up from startDir to the nearest directory containing a
// Makefile, which by convention is the firmware source tree's root.
func findMakefileRoot(startDir string) (string, error) {
	dir := startDir
	for {
		if fileExists(filepath.Join(dir, "Makefile")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no Makefile found at or above %s", startDir)
		}
		dir = parent
	}
}

// fileExists reports whether path exists and is a regular file (not a directory).
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
