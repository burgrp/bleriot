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

	"site/bridge"
	"site/config"
	"site/engine"
	"site/inventory"
	"site/mcp2210"
	"site/node"
	"site/radio"
	"site/radio/mcpdongle"

	"github.com/burgrp/tinygo-drivers/pan211x"
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
	dongles    []string // each "scheme:selector,channel", e.g. "mcp2210:/dev/hidraw0,37"
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
	f.StringArrayVar(&o.dongles, "dongle", nil, "USB radio dongle as scheme:selector,channel (repeatable); "+
		"scheme selects the dongle type (e.g. \"mcp2210\"), selector is a /dev/hidraw* path or a USB serial, "+
		"e.g. mcp2210:/dev/hidraw0,37 or mcp2210:0001746423,37")
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

	// Each channel/dongle drives a single spreading factor; derive the map from
	// the inventory (Validate already proved each channel is uniform).
	sfByChannel, err := inv.SpreadFactorByChannel()
	if err != nil {
		return fmt.Errorf("inventory: %w", err)
	}

	if err := startDongles(ctx, eng, o.dongles, hubAddr, sfByChannel, logger); err != nil {
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

// dongleSpec is a parsed USB radio dongle: a typed device selector on an RF
// channel. scheme selects which dongle implementation opens the selector.
type dongleSpec struct {
	scheme   string
	selector string
	channel  uint8
}

// dongleOpener opens one dongle of a particular type and returns it as a
// radio.Dongle. New dongle types (e.g. a smart MCU-based dongle) are added by
// registering another opener in dongleOpeners.
type dongleOpener func(selector string, channel uint8, spreadFactor config.SpreadFactor, hubAddr [node.AddrLen]byte) (radio.Dongle, error)

// dongleOpeners maps a scheme to its opener. To support a new dongle type, add
// an entry here; the flag parsing and startup wiring need no other changes.
var dongleOpeners = map[string]dongleOpener{
	"mcp2210": openMCP2210,
}

// openMCP2210 opens an MCP2210 USB-to-SPI bridge by selector and brings up its
// PAN211x radio on the given channel and spreading factor.
func openMCP2210(selector string, channel uint8, spreadFactor config.SpreadFactor, hubAddr [node.AddrLen]byte) (radio.Dongle, error) {
	dev, err := mcp2210.Open(selector)
	if err != nil {
		return nil, err
	}
	return mcpdongle.Open(dev, channel, pan211x.SpreadFactor(spreadFactor), hubAddr)
}

// startDongles parses each --dongle flag, opens its dongle via the registered
// opener for its scheme, brings up the radio on the requested channel and the
// channel's spreading factor (from the inventory), and registers it with the
// engine. At least one dongle is required.
func startDongles(ctx context.Context, eng *engine.Engine, flags []string, hubAddr [node.AddrLen]byte, sfByChannel map[uint8]config.SpreadFactor, logger *slog.Logger) error {
	specs, err := parseDongles(flags)
	if err != nil {
		return err
	}
	if len(specs) == 0 {
		return fmt.Errorf("no radio dongle: set at least one --dongle scheme:selector,channel")
	}
	for _, s := range specs {
		sf, ok := sfByChannel[s.channel]
		if !ok {
			// No inventory node uses this channel; fall back to the default factor
			// and warn, since such a dongle has nothing to talk to.
			logger.Warn("dongle channel has no inventory nodes; using default spreading factor",
				"channel", s.channel, "spreadFactor", sf)
		}
		d, err := dongleOpeners[s.scheme](s.selector, s.channel, sf, hubAddr)
		if err != nil {
			return fmt.Errorf("dongle %q: %w", s.selector, err)
		}
		eng.AddRadio(ctx, s.channel, radio.New(ctx, d))
		logger.Info("radio dongle ready", "type", s.scheme, "selector", s.selector, "channel", s.channel, "spreadFactor", sf)
	}
	return nil
}

// parseDongles parses "scheme:selector,channel" entries. The channel is split
// off after the last comma so device paths or serials containing commas are
// preserved; the "scheme:" prefix (before the first colon) selects the dongle
// type and is required — there is no default.
func parseDongles(flags []string) ([]dongleSpec, error) {
	specs := make([]dongleSpec, 0, len(flags))
	for _, f := range flags {
		i := strings.LastIndex(f, ",")
		if i < 0 {
			return nil, fmt.Errorf("dongle %q: expected scheme:selector,channel", f)
		}
		target := f[:i]
		ch, err := strconv.ParseUint(f[i+1:], 10, 8)
		if err != nil {
			return nil, fmt.Errorf("dongle %q: invalid channel: %w", f, err)
		}
		j := strings.Index(target, ":")
		if j < 0 {
			return nil, fmt.Errorf("dongle %q: missing scheme: prefix (e.g. mcp2210:)", f)
		}
		scheme, selector := target[:j], target[j+1:]
		if selector == "" {
			return nil, fmt.Errorf("dongle %q: empty selector", f)
		}
		if _, ok := dongleOpeners[scheme]; !ok {
			return nil, fmt.Errorf("dongle %q: unknown dongle type %q", f, scheme)
		}
		specs = append(specs, dongleSpec{scheme: scheme, selector: selector, channel: uint8(ch)})
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
