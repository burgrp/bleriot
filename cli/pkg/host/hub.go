package host

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/burgrp/reg/pkg/client"
	clientfactory "github.com/burgrp/reg/pkg/client/factory"
	"github.com/spf13/cobra"

	"cli/pkg/bridge"
	"cli/pkg/engine"
	"cli/pkg/inventory"
	"cli/pkg/mcp2210"
	"cli/pkg/node"
	"cli/pkg/radio"
)

// hubOptions holds the runtime/deploy settings for the hub, all sourced from
// command-line flags (not the inventory).
type hubOptions struct {
	registry   string
	hubAddress string
	timeout    time.Duration
	retries    int
	refresh    time.Duration
	ttl        time.Duration
	dongles    []string // each "selector,channel", e.g. "/dev/hidraw0,37"
}

// newHubCmd builds the "hub" subcommand: bridge the inventory's nodes to the
// Registry. Every setting that varies by deployment is a flag.
func newHubCmd(inv inventory.Inventory) *cobra.Command {
	var o hubOptions
	cmd := &cobra.Command{
		Use:   "hub",
		Short: "Bridge the inventory's RF nodes to the Registry",
		Long: "Run the BleRiot host hub: open one or more USB radio dongles " +
			"(each an MCP2210 USB-to-SPI bridge driving a single PAN211x radio) and " +
			"bridge every node in the inventory to the external Registry service.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHub(cmd.Context(), inv, o, slog.Default())
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.registry, "registry", "", "Registry base URL (falls back to the REGISTRY environment variable)")
	f.StringVar(&o.hubAddress, "hub-address", "FFFFFF01", "hub RF address (8 hex digits)")
	f.DurationVar(&o.timeout, "timeout", 50*time.Millisecond, "per-attempt response wait")
	f.IntVar(&o.retries, "retries", 3, "retransmissions after the first attempt")
	f.DurationVar(&o.refresh, "refresh", 15*time.Second, "how often to re-WATCH active subscriptions")
	f.DurationVar(&o.ttl, "ttl", 30*time.Second, "Registry provider TTL for each register")
	f.StringArrayVar(&o.dongles, "dongle", nil, "USB radio dongle as selector,channel (repeatable); "+
		"selector is a /dev/hidraw* path or a USB serial, e.g. /dev/hidraw0,37")
	return cmd
}

func runHub(ctx context.Context, inv inventory.Inventory, o hubOptions, logger *slog.Logger) error {
	if err := inv.Validate(); err != nil {
		return fmt.Errorf("inventory: %w", err)
	}
	hubAddr, err := node.ParseAddress(o.hubAddress)
	if err != nil {
		return fmt.Errorf("hub-address: %w", err)
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	// Cancelling the context makes each radio's receive loop stop.
	defer stop()

	eng := engine.New(engine.Options{
		HubAddr:         hubAddr,
		Timeout:         o.timeout,
		Retries:         o.retries,
		RefreshInterval: o.refresh,
	})
	go eng.Run(ctx)

	if err := startDongles(ctx, eng, o.dongles, hubAddr, logger); err != nil {
		return err
	}

	nodes, err := buildNodes(inv, eng)
	if err != nil {
		return err
	}

	regClient, regURL, err := newRegistry(o.registry)
	if err != nil {
		return err
	}
	logger.Info("using registry", "url", regURL)

	br := bridge.New(eng, regClient, o.ttl, bridge.WithLogger(logger))
	for _, n := range nodes {
		br.ServeNode(ctx, n)
		logger.Info("serving node", "node", n.Name, "registers", len(n.Registers), "channel", n.Channel)
	}

	logger.Info("hub running; press Ctrl-C to stop", "nodes", len(nodes))
	<-ctx.Done()
	logger.Info("hub shutting down")
	return nil
}

// buildNodes converts every inventory instance into a node and registers it with
// the engine.
func buildNodes(inv inventory.Inventory, eng *engine.Engine) ([]*node.Node, error) {
	nodes := make([]*node.Node, 0, len(inv))
	for _, inst := range inv {
		n, err := buildNode(inst)
		if err != nil {
			return nil, fmt.Errorf("node %q: %w", inst.Name, err)
		}
		if err := eng.AddNode(n); err != nil {
			return nil, fmt.Errorf("node %q: %w", inst.Name, err)
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

// dongleSpec is a parsed USB radio dongle: a device selector on an RF channel.
type dongleSpec struct {
	selector string
	channel  uint8
}

// startDongles parses each --dongle flag, opens the MCP2210, brings up its
// PAN211x radio on the requested channel, and registers it with the engine. At
// least one dongle is required.
func startDongles(ctx context.Context, eng *engine.Engine, flags []string, hubAddr [node.AddrLen]byte, logger *slog.Logger) error {
	specs, err := parseDongles(flags)
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		return fmt.Errorf("no radio dongle: set at least one --dongle selector,channel")
	}
	for _, s := range specs {
		dev, err := mcp2210.Open(s.selector)
		if err != nil {
			return fmt.Errorf("dongle %q: %w", s.selector, err)
		}
		r, err := radio.New(ctx, dev, s.channel, hubAddr)
		if err != nil {
			dev.Close()
			return fmt.Errorf("dongle %q: %w", s.selector, err)
		}
		eng.AddRadio(ctx, s.channel, r)
		logger.Info("radio dongle ready", "selector", s.selector, "channel", s.channel)
	}
	return nil
}

// parseDongles parses "selector,channel" entries. The channel is split off after
// the last comma so device paths or serials containing commas are preserved.
func parseDongles(flags []string) ([]dongleSpec, error) {
	specs := make([]dongleSpec, 0, len(flags))
	for _, f := range flags {
		i := strings.LastIndex(f, ",")
		if i < 0 {
			return nil, fmt.Errorf("dongle %q: expected selector,channel", f)
		}
		selector := f[:i]
		if selector == "" {
			return nil, fmt.Errorf("dongle %q: empty selector", f)
		}
		ch, err := strconv.ParseUint(f[i+1:], 10, 8)
		if err != nil {
			return nil, fmt.Errorf("dongle %q: invalid channel: %w", f, err)
		}
		specs = append(specs, dongleSpec{selector: selector, channel: uint8(ch)})
	}
	return specs, nil
}

// newRegistry creates the registry client and returns it along with the resolved
// registry URL (from the flag, or the REGISTRY environment variable when the
// flag is empty).
func newRegistry(url string) (client.Client, string, error) {
	if url == "" {
		url = os.Getenv("REGISTRY")
		if url == "" {
			return nil, "", fmt.Errorf("no registry address: set --registry or the REGISTRY environment variable")
		}
	}
	c, err := clientfactory.NewClient(url)
	if err != nil {
		return nil, url, err
	}
	return c, url, nil
}
