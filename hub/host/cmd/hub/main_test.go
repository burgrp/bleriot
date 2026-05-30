package main

import (
	"os"
	"path/filepath"
	"testing"

	"hub/host/engine"
	"hub/host/node"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hub.json")
	content := `{
	  "registry": "http://localhost:8080",
	  "hubAddress": "FFFFFF01",
	  "timeoutMs": 50,
	  "retries": 3,
	  "refreshSeconds": 15,
	  "ttlSeconds": 30,
	  "baud": 115200,
	  "ports": [{ "device": "/dev/ttyACM0", "channel": 37 }],
	  "nodes": "nodes"
	}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, baseDir, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Registry != "http://localhost:8080" || cfg.HubAddress != "FFFFFF01" {
		t.Errorf("unexpected top-level fields: %+v", cfg)
	}
	if cfg.RefreshSec != 15 || cfg.TTLSeconds != 30 || cfg.Retries != 3 || cfg.TimeoutMs != 50 {
		t.Errorf("unexpected scalars: %+v", cfg)
	}
	if len(cfg.Ports) != 1 || cfg.Ports[0].Device != "/dev/ttyACM0" || cfg.Ports[0].Channel != 37 {
		t.Errorf("unexpected ports: %+v", cfg.Ports)
	}
	if cfg.NodesDir != "nodes" {
		t.Errorf("unexpected nodes: %q", cfg.NodesDir)
	}
	if baseDir != dir {
		t.Errorf("baseDir = %q, want %q", baseDir, dir)
	}
}

func TestLoadConfig_Errors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		if _, _, err := loadConfig(filepath.Join(t.TempDir(), "nope.json")); err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.json")
		if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := loadConfig(path); err == nil {
			t.Fatal("expected error for invalid json")
		}
	})
}

func TestNewRegistry(t *testing.T) {
	t.Run("explicit url", func(t *testing.T) {
		c, url, err := newRegistry("http://example:8080")
		if err != nil {
			t.Fatalf("newRegistry: %v", err)
		}
		if c == nil {
			t.Fatal("nil client")
		}
		if url != "http://example:8080" {
			t.Errorf("url = %q", url)
		}
	})

	t.Run("env fallback", func(t *testing.T) {
		t.Setenv("REGISTRY", "http://from-env:9000")
		c, url, err := newRegistry("")
		if err != nil {
			t.Fatalf("newRegistry: %v", err)
		}
		if c == nil {
			t.Fatal("nil client")
		}
		if url != "http://from-env:9000" {
			t.Errorf("url = %q, want env value", url)
		}
	})

	t.Run("missing", func(t *testing.T) {
		t.Setenv("REGISTRY", "")
		if _, _, err := newRegistry(""); err == nil {
			t.Fatal("expected error when no registry address is configured")
		}
	})

	t.Run("invalid scheme", func(t *testing.T) {
		if _, _, err := newRegistry("ftp://nope"); err == nil {
			t.Fatal("expected error for unsupported scheme")
		}
	})
}

const sampleDescriptor = `{
  "version": "0x1A2B3C4D",
  "metadata": {},
  "registers": [
    { "id": 4660, "name": "outdoor.temperature", "class": "thermometer", "instance": "outdoor",
      "type": "float", "multiplier": 1, "divider": 100, "metadata": {} }
  ]
}`

func TestLoadNodes(t *testing.T) {
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "descriptors"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(base, "nodes"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(base, rel), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("descriptors/thermo.json", sampleDescriptor)
	write("nodes/outdoor.json", `{ "descriptor": "../descriptors/thermo.json", "channel": 37, "address": "CCA00002", "key": "00112233445566778899AABBCCDDEEFF" }`)
	write("nodes/garage.json", `{ "descriptor": "../descriptors/thermo.json", "channel": 11, "address": "CCA00003", "key": "00112233445566778899AABBCCDDEEFF" }`)
	// A non-JSON file in the directory must be ignored.
	write("nodes/README.txt", "ignore me")

	eng := engine.New(engine.Options{HubAddr: [node.AddrLen]byte{0xFF, 0xFF, 0xFF, 0x01}})
	nodes, err := loadNodes(config{NodesDir: "nodes"}, base, eng)
	if err != nil {
		t.Fatalf("loadNodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}
	channels := map[string]uint8{}
	for _, n := range nodes {
		channels[n.Name] = n.Channel
		if len(n.Registers) != 1 {
			t.Errorf("node %q: registers=%d, want 1", n.Name, len(n.Registers))
		}
	}
	if channels["outdoor"] != 37 || channels["garage"] != 11 {
		t.Errorf("channels = %v, want outdoor=37 garage=11", channels)
	}
}

func TestLoadNodes_Errors(t *testing.T) {
	eng := func() *engine.Engine {
		return engine.New(engine.Options{HubAddr: [node.AddrLen]byte{0xFF, 0xFF, 0xFF, 0x01}})
	}

	t.Run("missing nodesDir", func(t *testing.T) {
		if _, err := loadNodes(config{}, t.TempDir(), eng()); err == nil {
			t.Fatal("expected error when nodesDir is empty")
		}
	})

	t.Run("directory not found", func(t *testing.T) {
		if _, err := loadNodes(config{NodesDir: "nope"}, t.TempDir(), eng()); err == nil {
			t.Fatal("expected error for missing directory")
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		base := t.TempDir()
		if err := os.Mkdir(filepath.Join(base, "nodes"), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := loadNodes(config{NodesDir: "nodes"}, base, eng()); err == nil {
			t.Fatal("expected error for empty directory")
		}
	})
}
