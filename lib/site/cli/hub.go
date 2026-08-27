package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/burgrp/reg/pkg/client"
	clientfactory "github.com/burgrp/reg/pkg/client/factory"
	wirerest "github.com/burgrp/reg/pkg/wire/rest"
	"github.com/spf13/cobra"

	"github.com/burgrp/bleriot/lib/shared/config"
	"github.com/burgrp/bleriot/lib/shared/inventory"
	"github.com/burgrp/bleriot/lib/site/bridge"
	"github.com/burgrp/bleriot/lib/site/engine"
	"github.com/burgrp/bleriot/lib/site/mcp2210"
	"github.com/burgrp/bleriot/lib/site/node"
	"github.com/burgrp/bleriot/lib/site/radio"
	"github.com/burgrp/bleriot/lib/site/radio/mcpdongle"

	"github.com/burgrp/tinygo-drivers/pan211x"
)

// hubOptions holds the runtime/deploy settings for the hub, all sourced from
// command-line flags (not the inventory).
type hubOptions struct {
	registry     string
	hubAddress   string
	timeout      time.Duration
	retries      int
	refresh      time.Duration
	ttl          time.Duration
	diagnostics  string // registry namespace prefix for diagnostic registers; empty disables them
	diagInterval time.Duration
}

// newHubCmd builds the "hub" subcommand: bridge the inventory's nodes to the
// Registry. Every setting that varies by deployment is a flag.
func newHubCmd(inv inventory.Inventory) *cobra.Command {
	var o hubOptions
	cmd := &cobra.Command{
		Use:   "hub",
		Short: "Bridge the inventory's RF nodes to the Registry",
		Long: "Run the BleRiot host hub: discover the connected USB radio dongles " +
			"(each an MCP2210 USB-to-SPI bridge driving a single PAN211x radio), assign " +
			"them automatically to the RF channels the inventory uses, and bridge every " +
			"node in the inventory to the external Registry service.",
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
	f.StringVar(&o.diagnostics, "diagnostics", "", "publish hub-synthesised diagnostic registers under this "+
		"registry namespace prefix (e.g. \"diag\"); empty disables them")
	f.DurationVar(&o.diagInterval, "diag-interval", bridge.DefaultDiagnosticInterval, "diagnostic Registry batch publication interval")
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

	dongles, err := startDongles(ctx, eng, hubAddr, sfByChannel, inv.ChannelNames(), logger)
	if err != nil {
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

	if o.diagnostics != "" {
		diagRegistry := wirerest.NewProviderClient(regURL)
		diag := bridge.NewDiagnostics(eng, diagRegistry, o.diagnostics, o.diagInterval, o.ttl, bridge.WithDiagLogger(logger))
		diagNodes := make([]bridge.DiagNode, len(nodes))
		for i, n := range nodes {
			diagNodes[i] = bridge.DiagNode{Name: n.Name, Addr: n.Address}
		}
		diag.Serve(ctx, diagNodes, dongles)
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

// dongleOpener opens one dongle of a particular type, by selector, and brings up
// its radio on the given channel and spreading factor.
type dongleOpener func(selector string, channel uint8, spreadFactor config.SpreadFactor, hubAddr [node.AddrLen]byte) (radio.Dongle, error)

// dongleType describes one supported dongle scheme: how to discover every
// connected device of that type, how to open one by selector, and how to learn
// its reply guard (lib/README.md §6) without opening it (the guard depends only
// on the channel's spreading factor, so the engine can validate it up front).
type dongleType struct {
	// scheme names the dongle type in logs and diagnostics, e.g. "mcp2210".
	scheme string
	// discover returns a stable selector for every connected device of this type.
	// Each selector round-trips through open and is stable enough (a USB serial
	// where possible) that a reconnecting supervisor re-finds the same physical
	// device after a replug.
	discover func() ([]string, error)
	open     dongleOpener
	guard    func(spreadFactor config.SpreadFactor) time.Duration
}

// dongleTypes lists every supported dongle type. The hub discovers all connected
// devices across all of them and assigns them to channels. To support a new
// dongle, add an entry here; nothing else changes.
var dongleTypes = []dongleType{
	{scheme: "mcp2210", discover: mcp2210.Discover, open: openMCP2210, guard: mcp2210Guard},
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

// mcp2210Guard reports an MCP2210 dongle's reply guard for a spreading factor,
// without opening a device.
func mcp2210Guard(spreadFactor config.SpreadFactor) time.Duration {
	return mcpdongle.ReplyGuard(pan211x.SpreadFactor(spreadFactor))
}

// dongleAssigner hands connected radio dongles to channels on demand. Every
// channel's supervised radio asks it to claim a dongle each time it (re)opens;
// the assigner discovers the connected devices across all dongle types and lends
// out one that no other channel already holds. Dongles are interchangeable, so
// any free device serves any orphan channel, and a device that disconnects is
// released back to the pool to be reassigned — possibly to a different channel —
// when it returns.
type dongleAssigner struct {
	types []dongleType

	mu      sync.Mutex
	claimed map[string]bool // scheme-qualified selector -> in use by some channel
}

func newDongleAssigner(types []dongleType) *dongleAssigner {
	return &dongleAssigner{types: types, claimed: make(map[string]bool)}
}

// errNoFreeDongle reports that no connected dongle is currently free to serve a
// channel. The channel's supervisor treats it like any other open failure: it
// stays offline and retries, so a dongle plugged in later is picked up.
var errNoFreeDongle = errors.New("no unassigned radio dongle connected")

// claim opens a connected dongle that no other channel is using and brings it up
// on the given channel and spreading factor. The returned dongle releases its
// claim when closed, so a drop frees it for reassignment. It returns
// errNoFreeDongle when every connected dongle is already claimed (or none is
// connected).
func (a *dongleAssigner) claim(channel uint8, sf config.SpreadFactor, hubAddr [node.AddrLen]byte) (radio.Dongle, error) {
	for {
		dt, sel, key, ok, err := a.reserve()
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errNoFreeDongle
		}
		d, err := dt.open(sel, channel, sf, hubAddr)
		if err != nil {
			// Reserved but could not open (vanished, busy, no permission): release
			// the reservation and try the next free device.
			a.unclaim(key)
			continue
		}
		return &claimedDongle{Dongle: d, release: func() { a.unclaim(key) }}, nil
	}
}

// reserve discovers the connected dongles across all types and atomically marks
// the first unclaimed one as claimed, returning how to open it. ok is false when
// every connected dongle is already claimed (or none is connected). Selectors
// within a type are sorted so assignment is deterministic.
func (a *dongleAssigner) reserve() (dt *dongleType, selector, key string, ok bool, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.types {
		t := &a.types[i]
		sels, e := t.discover()
		if e != nil {
			return nil, "", "", false, fmt.Errorf("discover %s dongles: %w", t.scheme, e)
		}
		sort.Strings(sels)
		for _, s := range sels {
			k := t.scheme + ":" + s
			if a.claimed[k] {
				continue
			}
			a.claimed[k] = true
			return t, s, k, true, nil
		}
	}
	return nil, "", "", false, nil
}

// unclaim releases a previously reserved dongle back to the pool.
func (a *dongleAssigner) unclaim(key string) {
	a.mu.Lock()
	delete(a.claimed, key)
	a.mu.Unlock()
}

// claimedDongle wraps an assigned dongle so closing it (after a drop or at
// shutdown) releases its claim, letting the same physical device be reassigned.
type claimedDongle struct {
	radio.Dongle
	once    sync.Once
	release func()
}

func (c *claimedDongle) ReceiveWithError(buf []byte) (int, bool, error) {
	if errorDongle, ok := c.Dongle.(radio.ReceiveErrorDongle); ok {
		return errorDongle.ReceiveWithError(buf)
	}
	n, received := c.Dongle.Receive(buf)
	return n, received, nil
}

func (c *claimedDongle) Close() error {
	c.once.Do(c.release)
	return c.Dongle.Close()
}

// startDongles registers one supervised radio per RF channel the inventory uses
// and assigns connected dongles to those channels. The hub always starts — even
// with no dongle connected: each channel's radio is registered with the engine
// immediately (its reply guard is known from the channel's spreading factor, not
// the hardware) and stays offline until a dongle becomes available. Dongles are
// assigned dynamically as they appear and released for reassignment when they
// drop, so plugging a dongle in brings up an orphan channel and unplugging it
// frees the device for another. It returns one bridge.DiagDongle per channel
// (labelled by channel name) for per-channel dongle diagnostics.
func startDongles(ctx context.Context, eng *engine.Engine, hubAddr [node.AddrLen]byte, sfByChannel map[uint8]config.SpreadFactor, chNames map[uint8]string, logger *slog.Logger) ([]bridge.DiagDongle, error) {
	assigner := newDongleAssigner(dongleTypes)
	diag := make([]bridge.DiagDongle, 0, len(chNames))
	for _, ch := range sortedChannels(chNames) {
		ch := ch // capture per iteration for the opener closure
		sf := sfByChannel[ch]
		open := func() (radio.Dongle, error) {
			return assigner.claim(ch, sf, hubAddr)
		}
		log := logger.With("channel", ch, "channelName", chNames[ch])
		rec := radio.NewReconnecting(ctx, open, maxGuard(sf), radio.DefaultReconnectBackoff, log)
		if err := eng.AddRadio(ctx, ch, radio.New(ctx, rec)); err != nil {
			rec.Close()
			return nil, fmt.Errorf("channel %d: %w", ch, err)
		}
		diag = append(diag, bridge.DiagDongle{Name: chNames[ch], Stats: rec.Stats})
	}
	logger.Info("radio channels ready; dongles are assigned as they connect", "channels", len(chNames))
	return diag, nil
}

// maxGuard is the largest reply guard (lib/README.md §6) any supported dongle
// type needs for a spreading factor. Because a channel's dongle is assigned
// dynamically, the node must defer its reply long enough for whichever dongle
// type ends up serving the channel; the maximum satisfies them all. The engine
// validates this guard against its timeout up front, before any device connects.
func maxGuard(sf config.SpreadFactor) time.Duration {
	var g time.Duration
	for i := range dongleTypes {
		if d := dongleTypes[i].guard(sf); d > g {
			g = d
		}
	}
	return g
}

// sortedChannels returns the inventory's channel numbers in ascending order, for
// a deterministic dongle assignment.
func sortedChannels(chNames map[uint8]string) []uint8 {
	chs := make([]uint8, 0, len(chNames))
	for ch := range chNames {
		chs = append(chs, ch)
	}
	sort.Slice(chs, func(i, j int) bool { return chs[i] < chs[j] })
	return chs
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
