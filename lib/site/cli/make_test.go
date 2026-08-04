package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/burgrp/bleriot/lib/shared/inventory"
)

func TestBuildMakeArgs(t *testing.T) {
	chip := inventory.Chip{
		TinygoTarget: "py32f030_64k_8k",
		PyocdTarget:  "py32f030x8",
		CmsisPack:    "PY32F030",
	}
	got := buildMakeArgs("/src/node", chip, []string{"flash", "-j2"})
	want := []string{
		"-C", "/src/node",
		"flash", "-j2",
		"BLERIOT_MAKE=1",
		"TARGET_TINYGO=py32f030_64k_8k",
		"TARGET_PYOCD=py32f030x8",
		"CMSIS_PACK=PY32F030",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg %d: got %q, want %q (full %v)", i, got[i], want[i], got)
		}
	}
}

func TestBuildMakeArgsOmitsEmptyChipFields(t *testing.T) {
	got := buildMakeArgs("/src", inventory.Chip{PyocdTarget: "stm32g030x6"}, nil)
	want := []string{"-C", "/src", "BLERIOT_MAKE=1", "TARGET_PYOCD=stm32g030x6"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg %d: got %q, want %q (full %v)", i, got[i], want[i], got)
		}
	}
}

func TestFindMakefileRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "node")
	nested := filepath.Join(root, "spec", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte("build:\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := findMakefileRoot(nested)
	if err != nil {
		t.Fatalf("findMakefileRoot: %v", err)
	}
	if got != root {
		t.Fatalf("got %q, want %q", got, root)
	}
}

func TestFindMakefileRootNotFound(t *testing.T) {
	dir := t.TempDir()
	if _, err := findMakefileRoot(dir); err == nil {
		t.Fatal("expected error when no Makefile exists above the start dir")
	}
}

func TestFindMakefileRootIgnoresDirectoryNamedMakefile(t *testing.T) {
	base := t.TempDir()
	// A directory (not a file) named "Makefile" must not be treated as a match.
	if err := os.MkdirAll(filepath.Join(base, "Makefile"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := findMakefileRoot(base); err == nil {
		t.Fatal("expected error: a directory named Makefile is not a Makefile")
	}
}

func TestInferRootNilConfig(t *testing.T) {
	_, err := inferRoot(inventory.Instance{Name: "n"})
	if err == nil {
		t.Fatal("expected error for nil Config")
	}
}

func TestSplitInstanceArgs(t *testing.T) {
	inv := inventory.Inventory{{Name: "bob"}, {Name: "kitchen"}}
	tests := []struct {
		args     []string
		wantName string
		wantMake []string
	}{
		{[]string{"bob", "flash"}, "bob", []string{"flash"}},
		{[]string{"flash"}, "", []string{"flash"}},
		{[]string{"kitchen"}, "kitchen", nil},
		{nil, "", nil},
	}
	for _, tt := range tests {
		name, mk := splitInstanceArgs(inv, tt.args)
		if name != tt.wantName {
			t.Errorf("args %v: name = %q, want %q", tt.args, name, tt.wantName)
		}
		if strings.Join(mk, "\x00") != strings.Join(tt.wantMake, "\x00") {
			t.Errorf("args %v: make = %v, want %v", tt.args, mk, tt.wantMake)
		}
	}
}

func TestWriteProvisioning(t *testing.T) {
	root := t.TempDir()
	inst := inventory.Instance{Name: "n", Channel: inventory.Channel{Number: 37}}

	if err := writeProvisioning(root, inst); err != nil {
		t.Fatalf("writeProvisioning: %v", err)
	}
	dst := filepath.Join(root, "main_gen.go")
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}
	if !strings.Contains(string(data), "DO NOT EDIT") {
		t.Fatalf("generated file missing header:\n%s", data)
	}

	// An identical re-render must not rewrite the file (mtime stays old), so an
	// unrelated make target does not force a rebuild.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(dst, old, old); err != nil {
		t.Fatal(err)
	}
	if err := writeProvisioning(root, inst); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(dst); info.ModTime().After(time.Now().Add(-time.Minute)) {
		t.Fatal("file was rewritten despite identical content")
	}

	// Changed content must rewrite the file (mtime becomes recent).
	inst.Channel.Number = 38
	if err := writeProvisioning(root, inst); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(dst); info.ModTime().Before(time.Now().Add(-time.Minute)) {
		t.Fatal("file was not rewritten after content change")
	}
}
