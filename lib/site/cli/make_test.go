package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/burgrp/bleriot/lib/shared/inventory"
)

func TestBuildMakeArgs(t *testing.T) {
	chip := inventory.Chip{
		TinygoTarget: "py32f030_64k_8k",
		PyocdTarget:  "py32f030x8",
		CmsisPack:    "PY32F030",
	}
	got := buildMakeArgs("/src/node", "/usr/bin/bleriot gen bob", chip, []string{"flash", "-j2"})
	want := []string{
		"-C", "/src/node",
		"flash", "-j2",
		"GEN=/usr/bin/bleriot gen bob",
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
	got := buildMakeArgs("/src", "gen", inventory.Chip{PyocdTarget: "stm32g030x6"}, nil)
	want := []string{"-C", "/src", "GEN=gen", "TARGET_PYOCD=stm32g030x6"}
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
