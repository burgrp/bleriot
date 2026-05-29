// Command hub is the BleRiot host bridge. It connects one or more "dumb radio
// modems" (each a serial port driving a single PAN211x radio) to the external
// Registry service: every node register is published as a Registry provider and
// consumer change requests are turned into BleRiot SET operations.
//
// Configuration is a JSON file (see -config). Example:
//
//	{
//	  "registry": "http://localhost:8080",
//	  "hubAddress": "FFFFFF01",
//	  "timeoutMs": 50,
//	  "retries": 3,
//	  "refreshSeconds": 15,
//	  "ttlSeconds": 30,
//	  "baud": 115200,
//	  "ports": [
//	    { "device": "/dev/ttyUSB0", "channel": 37 }
//	  ],
//	  "nodesDir": "nodes"
//	}
//
// "nodesDir" points to a directory of per-device node files. Each *.json file
// there is a thin instance file naming a shared descriptor plus the device's
// provisioned identity, and the file's base name is the node name:
//
//	{
//	  "descriptor": "../descriptors/thermo.json",
//	  "channel": 37,
//	  "address": "CCA00002",
//	  "key": "00112233445566778899AABBCCDDEEFF"
//	}
//
// Provisioning a new device means dropping another file into nodesDir; the hub
// config never needs editing. Paths in the config are resolved relative to the
// config file's directory, and a node file's "descriptor" path is resolved
// relative to that node file. If "registry" is empty the REGISTRY environment
// variable is used.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/burgrp/reg/pkg/client"
	clientfactory "github.com/burgrp/reg/pkg/client/factory"
	"github.com/lmittmann/tint"
	"go.bug.st/serial"

	"hub/host/bridge"
	"hub/host/engine"
	"hub/host/modem"
	"hub/host/node"
)

type config struct {
	Registry   string       `json:"registry"`
	HubAddress string       `json:"hubAddress"`
	TimeoutMs  int          `json:"timeoutMs"`
	Retries    int          `json:"retries"`
	RefreshSec int          `json:"refreshSeconds"`
	TTLSeconds int          `json:"ttlSeconds"`
	Baud       int          `json:"baud"`
	Ports      []portConfig `json:"ports"`
	NodesDir   string       `json:"nodesDir"`
}

type portConfig struct {
	Device  string `json:"device"`
	Baud    int    `json:"baud"`
	Channel uint8  `json:"channel"`
}

// nodeFile is a per-device node instance file living in nodesDir. The node name
// is the file's base name, not stored inside the file.
type nodeFile struct {
	Descriptor string `json:"descriptor"`
	Channel    uint8  `json:"channel"`
	Address    string `json:"address"`
	Key        string `json:"key"`
}

func main() {
	cfgPath := flag.String("config", "hub.json", "path to the hub configuration file")
	debug := flag.Bool("debug", false, "enable debug logging (shows serial communication)")
	flag.Parse()

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level:      level,
		TimeFormat: time.TimeOnly,
	}))
	slog.SetDefault(logger)

	if err := run(*cfgPath, logger); err != nil {
		logger.Error("hub failed", "err", err)
		os.Exit(1)
	}
}

func run(cfgPath string, logger *slog.Logger) error {
	cfg, baseDir, err := loadConfig(cfgPath)
	if err != nil {
		return err
	}

	hubAddr, err := node.ParseAddress(cfg.HubAddress)
	if err != nil {
		return fmt.Errorf("hubAddress: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// Cancelling the context makes each radio Port close its transport and stop.
	defer stop()

	eng := engine.New(engine.Options{
		HubAddr:         hubAddr,
		Timeout:         time.Duration(cfg.TimeoutMs) * time.Millisecond,
		Retries:         cfg.Retries,
		RefreshInterval: time.Duration(cfg.RefreshSec) * time.Second,
	})
	go eng.Run(ctx)

	startPorts(ctx, cfg, eng, hubAddr, logger)

	nodes, err := loadNodes(cfg, baseDir, eng)
	if err != nil {
		return err
	}

	regClient, regURL, err := newRegistry(cfg.Registry)
	if err != nil {
		return err
	}
	logger.Info("using registry", "url", regURL)

	br := bridge.New(eng, regClient, time.Duration(cfg.TTLSeconds)*time.Second, bridge.WithLogger(logger))
	for _, n := range nodes {
		br.ServeNode(ctx, n)
		logger.Info("serving node", "node", n.Name, "registers", len(n.Registers), "channel", n.Channel)
	}

	logger.Info("hub running; press Ctrl-C to stop", "ports", len(cfg.Ports), "nodes", len(nodes))
	<-ctx.Done()
	logger.Info("hub shutting down")
	return nil
}

func loadConfig(path string) (config, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return config{}, "", fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return config{}, "", fmt.Errorf("parsing config %s: %w", path, err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return config{}, "", err
	}
	return cfg, filepath.Dir(abs), nil
}

// startPorts registers a self-healing radio Port for each configured serial
// device and registers it with the engine. Each Port opens its device lazily and
// reconnects on its own, so the hub starts even when a device is absent and
// recovers when it (re)appears. startPorts never blocks on hardware.
func startPorts(ctx context.Context, cfg config, eng *engine.Engine, hubAddr [node.AddrLen]byte, logger *slog.Logger) {
	for _, pc := range cfg.Ports {
		baud := pc.Baud
		if baud == 0 {
			baud = cfg.Baud
		}
		if baud == 0 {
			baud = 115200
		}
		device := pc.Device
		portLog := logger.With("port", device)
		open := func() (io.ReadWriteCloser, error) {
			return serial.Open(device, &serial.Mode{BaudRate: baud})
		}
		p := modem.NewPort(modem.PortConfig{
			Open:       open,
			Channel:    pc.Channel,
			Addr:       hubAddr,
			RecvBuffer: 32,
			Logger:     portLog,
		})
		go p.Run(ctx)
		eng.AddRadio(ctx, pc.Channel, p)
		logger.Info("radio registered", "port", device, "channel", pc.Channel, "baud", baud)
	}
}

// loadNodes reads every *.json instance file in the configured nodes directory.
// Each file names a shared descriptor plus the device's RF channel and identity;
// the file's base name (without ".json") is the node name. The descriptor path
// is resolved relative to the instance file's own directory.
func loadNodes(cfg config, baseDir string, eng *engine.Engine) ([]*node.Node, error) {
	if cfg.NodesDir == "" {
		return nil, fmt.Errorf("config: nodesDir is required")
	}
	dir := cfg.NodesDir
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(baseDir, dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading nodes directory %s: %w", dir, err)
	}

	var nodes []*node.Node
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		nfPath := filepath.Join(dir, e.Name())
		nf, err := loadNodeFile(nfPath)
		if err != nil {
			return nil, err
		}
		descPath := nf.Descriptor
		if !filepath.IsAbs(descPath) {
			descPath = filepath.Join(dir, descPath)
		}
		desc, err := node.LoadDescriptorFile(descPath)
		if err != nil {
			return nil, fmt.Errorf("node %s: loading descriptor %s: %w", name, descPath, err)
		}
		id, err := node.ParseIdentity(nf.Address, nf.Key)
		if err != nil {
			return nil, fmt.Errorf("node %s identity: %w", name, err)
		}
		n := node.NewNode(name, nf.Channel, desc, id)
		if err := eng.AddNode(n); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no node files (*.json) found in %s", dir)
	}
	return nodes, nil
}

// loadNodeFile parses a single per-device node instance file.
func loadNodeFile(path string) (nodeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nodeFile{}, fmt.Errorf("reading node file %s: %w", path, err)
	}
	var nf nodeFile
	if err := json.Unmarshal(data, &nf); err != nil {
		return nodeFile{}, fmt.Errorf("parsing node file %s: %w", path, err)
	}
	if nf.Descriptor == "" {
		return nodeFile{}, fmt.Errorf("node file %s: descriptor is required", path)
	}
	return nf, nil
}

// newRegistry creates the registry client and returns it along with the
// resolved registry URL (from config, or the REGISTRY environment variable when
// the config leaves it empty).
func newRegistry(url string) (client.Client, string, error) {
	if url == "" {
		url = os.Getenv("REGISTRY")
		if url == "" {
			return nil, "", fmt.Errorf("no registry address: set \"registry\" in the config or the REGISTRY environment variable")
		}
	}
	c, err := clientfactory.NewClient(url)
	if err != nil {
		return nil, url, err
	}
	return c, url, nil
}
