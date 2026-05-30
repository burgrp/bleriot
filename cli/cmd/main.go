// Command bleriot is the command-line tool for the BleRiot RF register protocol.
//
// Subcommands:
//
//	bleriot hub        run the host hub bridge (RF nodes ↔ Registry)
//	bleriot generate   generate node code and hub descriptors from a spec
//	bleriot provision  (planned) provision a device's identity
package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
	"github.com/spf13/cobra"
)

// debug is the global verbosity flag, applied before any subcommand runs.
var debug bool

// newRootCmd builds the top-level "bleriot" command and wires its subcommands.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "bleriot",
		Short:         "BleRiot RF register protocol tooling",
		Long:          "bleriot is the command-line tool for the BleRiot RF register protocol: run the host hub, generate node code, and (soon) provision devices.",
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
	root.AddCommand(newHubCmd())
	root.AddCommand(newGenerateCmd())
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		slog.Error("command failed", "err", err)
		os.Exit(1)
	}
}
