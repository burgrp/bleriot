package main

import (
	"os"
	"path/filepath"
	"testing"
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
	  "nodes": [{ "descriptor": "nodes/thermo.json", "address": "CCA00002", "key": "00112233445566778899AABBCCDDEEFF" }]
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
	if len(cfg.Nodes) != 1 || cfg.Nodes[0].Descriptor != "nodes/thermo.json" {
		t.Errorf("unexpected nodes: %+v", cfg.Nodes)
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
