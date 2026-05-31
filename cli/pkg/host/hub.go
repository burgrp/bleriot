package host

import (
	"context"
	"fmt"
	"io"
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
	"go.bug.st/serial"

	"cli/pkg/bridge"
	"cli/pkg/engine"
	"cli/pkg/inventory"
	"cli/pkg/modem"
	"cli/pkg/node"
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
	baud       int
	ports      []string // each "device:channel", e.g. "/dev/ttyUSB0:37"
}

// newHubCmd builds the "hub" subcommand: bridge the inventory's nodes to the
// Registry. Every setting that varies by deployment is a flag.
func newHubCmd(inv inventory.Inventory) *cobra.Command {
	var o hubOptions
	cmd := &cobra.Command{
		Use:   "hub",
		Short: "Bridge the inventory's RF nodes to the Registry",
		Long: "Run the BleRiot host hub: connect one or more \"dumb radio modems\" " +
			"(each a serial port driving a single PAN211x radio) and bridge every " +
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
	f.IntVar(&o.baud, "baud", 115200, "default serial baud rate for ports")
	f.StringArrayVar(&o.ports, "port", nil, "radio port as device:channel (repeatable), e.g. /dev/ttyUSB0:37")
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
	ports, err := parsePorts(o.ports)
	if err != nil {
		return err
	}
	if len(ports) == 0 {
		return fmt.Errorf("no radio ports: pass at least one --port device:channel")
	}

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	// Cancelling the context makes each radio Port close its transport and stop.
	defer stop()

	eng := engine.New(engine.Options{
		HubAddr:         hubAddr,
		Timeout:         o.timeout,
		Retries:         o.retries,
		RefreshInterval: o.refresh,
	})
	go eng.Run(ctx)

	startPorts(ctx, ports, o.baud, eng, hubAddr, logger)

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

	logger.Info("hub running; press Ctrl-C to stop", "ports", len(ports), "nodes", len(nodes))
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

// port is a parsed radio port: a serial device on an RF channel.
type port struct {
	device  string
	channel uint8
}

// parsePorts parses --port "device:channel" specs. The device may itself contain
// colons (rare), so the channel is taken from the final colon-separated field.
func parsePorts(specs []string) ([]port, error) {
	ports := make([]port, 0, len(specs))
	for _, s := range specs {
		i := strings.LastIndex(s, ":")
		if i < 0 {
			return nil, fmt.Errorf("port %q: expected device:channel", s)
		}
		device, chStr := s[:i], s[i+1:]
		if device == "" {
			return nil, fmt.Errorf("port %q: empty device", s)
		}
		ch, err := strconv.ParseUint(chStr, 10, 8)
		if err != nil {
			return nil, fmt.Errorf("port %q: invalid channel %q: %w", s, chStr, err)
		}
		ports = append(ports, port{device: device, channel: uint8(ch)})
	}
	return ports, nil
}

// startPorts registers a self-healing radio Port for each serial device with the
// engine. Each Port opens its device lazily and reconnects on its own, so the
// hub starts even when a device is absent and recovers when it (re)appears.
func startPorts(ctx context.Context, ports []port, baud int, eng *engine.Engine, hubAddr [node.AddrLen]byte, logger *slog.Logger) {
	if baud == 0 {
		baud = 115200
	}
	for _, pc := range ports {
		device := pc.device
		portLog := logger.With("port", device)
		open := func() (io.ReadWriteCloser, error) {
			return serial.Open(device, &serial.Mode{BaudRate: baud})
		}
		p := modem.NewPort(modem.PortConfig{
			Open:       open,
			Channel:    pc.channel,
			Addr:       hubAddr,
			RecvBuffer: 32,
			Logger:     portLog,
		})
		go p.Run(ctx)
		eng.AddRadio(ctx, pc.channel, p)
		logger.Info("radio registered", "port", device, "channel", pc.channel, "baud", baud)
	}
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
