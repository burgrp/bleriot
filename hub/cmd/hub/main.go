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
//	  "nodes": [
//	    { "descriptor": "nodes/thermo.json",
//	      "address": "CCA00002",
//	      "key": "00112233445566778899AABBCCDDEEFF" }
//	  ]
//	}
//
// Paths in the config are resolved relative to the config file's directory.
// If "registry" is empty the REGISTRY environment variable is used.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/burgrp/reg/pkg/client"
	clientfactory "github.com/burgrp/reg/pkg/client/factory"
	"github.com/lmittmann/tint"
	"go.bug.st/serial"

	"hub/bridge"
	"hub/engine"
	"hub/modem"
	"hub/node"
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
	Nodes      []nodeConfig `json:"nodes"`
}

type portConfig struct {
	Device  string `json:"device"`
	Baud    int    `json:"baud"`
	Channel uint8  `json:"channel"`
}

type nodeConfig struct {
	Descriptor string `json:"descriptor"`
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

	var closers []serial.Port
	// On exit, cancel the context first so modems observe shutdown and stop
	// quietly, then close the serial ports to release their file descriptors.
	defer func() {
		stop()
		closeAll(closers)
	}()

	eng := engine.New(engine.Options{
		HubAddr:         hubAddr,
		Timeout:         time.Duration(cfg.TimeoutMs) * time.Millisecond,
		Retries:         cfg.Retries,
		RefreshInterval: time.Duration(cfg.RefreshSec) * time.Second,
	})
	go eng.Run(ctx)

	closers, err = startPorts(ctx, cfg, eng, hubAddr, logger)
	if err != nil {
		return err
	}

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
		logger.Info("serving node", "node", n.Node, "registers", len(n.Registers), "channel", n.Channel)
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

// startPorts opens each serial port, wraps it in a modem, configures its radio,
// and registers it with the engine. It returns the open ports so they can be
// closed on shutdown.
func startPorts(ctx context.Context, cfg config, eng *engine.Engine, hubAddr [node.AddrLen]byte, logger *slog.Logger) ([]serial.Port, error) {
	var ports []serial.Port
	for _, pc := range cfg.Ports {
		baud := pc.Baud
		if baud == 0 {
			baud = cfg.Baud
		}
		if baud == 0 {
			baud = 115200
		}
		p, err := serial.Open(pc.Device, &serial.Mode{BaudRate: baud})
		if err != nil {
			return ports, fmt.Errorf("opening serial port %s: %w", pc.Device, err)
		}
		ports = append(ports, p)

		modemLog := logger.With("port", pc.Device)
		m := modem.New(p, 32, modem.WithLogger(modemLog))
		go func(device string) {
			if err := m.Run(ctx); err != nil && ctx.Err() == nil {
				logger.Error("modem stopped", "port", device, "err", err)
			}
		}(pc.Device)

		if err := m.ConfigRadio(pc.Channel, hubAddr); err != nil {
			return ports, err
		}
		eng.AddRadio(ctx, pc.Channel, m)
		logger.Info("opened serial port", "port", pc.Device, "channel", pc.Channel, "baud", baud)
	}
	return ports, nil
}

func loadNodes(cfg config, baseDir string, eng *engine.Engine) ([]*node.Node, error) {
	var nodes []*node.Node
	for _, nc := range cfg.Nodes {
		path := nc.Descriptor
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		desc, err := node.LoadDescriptorFile(path)
		if err != nil {
			return nil, fmt.Errorf("loading node descriptor %s: %w", path, err)
		}
		id, err := node.ParseIdentity(nc.Address, nc.Key)
		if err != nil {
			return nil, fmt.Errorf("node %s identity: %w", path, err)
		}
		n := node.NewNode(desc, id)
		if err := eng.AddNode(n); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
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

func closeAll(ports []serial.Port) {
	for _, p := range ports {
		_ = p.Close()
	}
}
