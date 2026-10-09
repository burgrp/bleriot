package fwbuild

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/burgrp/bleriot/lib/shared/firmware"
	"github.com/burgrp/bleriot/lib/shared/inventory"
)

func TestTinyGoBuildArgsFanSwitch(t *testing.T) {
	chip := inventory.Chip{TinygoTarget: "py32f003x6"}
	profile := firmware.TinyGoProfile{
		Scheduler:        firmware.SchedulerTasks,
		StackSizeBytes:   1024,
		GarbageCollector: firmware.GCLeaking,
		Serial:           firmware.SerialRTT,
		SizeReport:       firmware.SizeReportHTML,
		PrintAllocs:      true,
	}
	want := []string{
		"build", "--stack-size", "1024b", "--size", "html", "--serial", "rtt",
		"--scheduler", "tasks", "-print-allocs=.", "--gc", "leaking",
		"--target", "py32f003x6", "-o", "image.elf", ".",
	}
	if got := tinyGoBuildArgs(chip, profile); !reflect.DeepEqual(got, want) {
		t.Fatalf("tinyGoBuildArgs = %v, want %v", got, want)
	}
}

func TestPyocdArgs(t *testing.T) {
	chip := inventory.Chip{PyocdTarget: "py32f030x8"}
	profile := firmware.PyOCDProfile{
		FrequencyHz: 100_000,
		LoadMode:    firmware.ConnectUnderReset,
		RTTMode:     firmware.ConnectAttach,
		GDBMode:     firmware.ConnectAttach,
	}
	if got, want := pyocdLoadArgs(chip, profile), []string{"load", "-t", "py32f030x8", "-f", "100000", "-M", "under-reset", "image.elf"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pyocdLoadArgs = %v, want %v", got, want)
	}
	if got, want := pyocdRTTArgs(chip, profile), []string{"rtt", "-t", "py32f030x8", "-f", "100000", "-M", "attach"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pyocdRTTArgs = %v, want %v", got, want)
	}
	if got, want := pyocdGDBArgs(chip, profile), []string{"gdb", "--target", "py32f030x8", "-f", "100000", "-M", "attach", "--semihosting", "--persist"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pyocdGDBArgs = %v, want %v", got, want)
	}
}

func TestFindModuleRoot(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "nested", "package")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := findModuleRoot(child)
	if err != nil {
		t.Fatalf("findModuleRoot: %v", err)
	}
	if got != root {
		t.Fatalf("findModuleRoot = %q, want %q", got, root)
	}
}

func TestSafeInstanceName(t *testing.T) {
	if got := safeInstanceName("basement/fan"); got != "basement%2Ffan" {
		t.Fatalf("safeInstanceName = %q", got)
	}
	if got := safeInstanceName("basement.fan"); got != "basement.fan" {
		t.Fatalf("safeInstanceName = %q", got)
	}
	if got := safeInstanceName("basement%2Ffan"); got != "basement%252Ffan" {
		t.Fatalf("safeInstanceName escaped literal = %q", got)
	}
	if safeInstanceName("basement/fan") == safeInstanceName("basement_fan") {
		t.Fatal("distinct instance names share a build directory")
	}
	if got := safeInstanceName(".."); got != "%2E%2E" {
		t.Fatalf("safeInstanceName traversal = %q", got)
	}
}

func TestBuildModuleGoFlags(t *testing.T) {
	got := buildModuleGoFlags("-trimpath -mod=vendor -tags=integration")
	want := "-trimpath -tags=integration -mod=mod"
	if got != want {
		t.Fatalf("buildModuleGoFlags = %q, want %q", got, want)
	}

	got = buildModuleGoFlags("-mod vendor -trimpath")
	want = "-trimpath -mod=mod"
	if got != want {
		t.Fatalf("buildModuleGoFlags split mod = %q, want %q", got, want)
	}
}

func TestBuildSteps(t *testing.T) {
	tests := []struct {
		name    string
		options BuildOptions
		want    []buildStep
	}{
		{name: "build", want: []buildStep{stepBuild}},
		{name: "disassembly", options: BuildOptions{Disassembly: true}, want: []buildStep{stepBuild, stepDisassembly}},
		{name: "flash", options: BuildOptions{Flash: true}, want: []buildStep{stepBuild, stepFlash}},
		{name: "rtt", options: BuildOptions{RTT: true}, want: []buildStep{stepBuild, stepRTT}},
		{name: "disassembly flash", options: BuildOptions{Disassembly: true, Flash: true}, want: []buildStep{stepBuild, stepDisassembly, stepFlash}},
		{name: "disassembly rtt", options: BuildOptions{Disassembly: true, RTT: true}, want: []buildStep{stepBuild, stepDisassembly, stepRTT}},
		{name: "flash rtt", options: BuildOptions{Flash: true, RTT: true}, want: []buildStep{stepBuild, stepFlash, stepRTT}},
		{name: "all", options: BuildOptions{Disassembly: true, Flash: true, RTT: true}, want: []buildStep{stepBuild, stepDisassembly, stepFlash, stepRTT}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := buildSteps(test.options); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("buildSteps(%+v) = %v, want %v", test.options, got, test.want)
			}
		})
	}
}

func TestBuildRunsEachRequestedStageOnceInOrder(t *testing.T) {
	moduleDir, logPath := fakeToolEnvironment(t, "tinygo", "arm-none-eabi-objdump", "pyocd")
	request := Request{
		InstanceName: "test.node",
		Source:       "package main\n",
		Manifest:     firmware.Manifest{Package: "example.com/acme/fw"},
		Chip: inventory.Chip{
			TinygoTarget: "tiny-target",
			PyocdTarget:  "probe-target",
		},
		ModuleDir: moduleDir,
	}
	options := BuildOptions{Disassembly: true, Flash: true, RTT: true}
	if err := Build(context.Background(), request, options); err != nil {
		t.Fatalf("Build: %v", err)
	}

	lines := readToolLog(t, logPath)
	wantPrefixes := []string{
		"tinygo build ",
		"arm-none-eabi-objdump -dS image.elf",
		"pyocd load -t probe-target image.elf",
		"pyocd rtt -t probe-target",
	}
	if len(lines) != len(wantPrefixes) {
		t.Fatalf("tool calls = %v, want %d calls", lines, len(wantPrefixes))
	}
	for index, prefix := range wantPrefixes {
		if !strings.HasPrefix(lines[index], prefix) {
			t.Fatalf("tool call %d = %q, want prefix %q", index, lines[index], prefix)
		}
	}
	buildDir := filepath.Join(moduleDir, buildStateDir, "firmware", "test.node")
	assertMode(t, buildDir, 0o700)
	assertMode(t, filepath.Join(buildDir, "go.mod"), 0o600)
	assertMode(t, filepath.Join(buildDir, "main.go"), 0o600)
	assertMode(t, filepath.Join(buildDir, imageName), 0o600)
	assertMode(t, filepath.Join(buildDir, "disassembly.txt"), 0o600)
}

func TestWriteBuildModuleSnapshotsVersionsAndWorkspaceReplacements(t *testing.T) {
	root := t.TempDir()
	dependency := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/site\n\ngo 1.25.2\n\nrequire example.com/dependency v0.0.0\n\nreplace example.com/dependency => "+filepath.ToSlash(dependency)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dependency, "go.mod"), []byte("module example.com/dependency\n\ngo 1.25.2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	buildDir := filepath.Join(root, ".bleriot", "firmware", "node")
	if err := os.MkdirAll(buildDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWORK", "off")
	if err := writeBuildModule(context.Background(), root, buildDir); err != nil {
		t.Fatalf("writeBuildModule: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(buildDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, want := range []string{
		"example.com/site v0.0.0",
		"example.com/dependency v0.0.0",
		"replace example.com/site => " + filepath.ToSlash(root),
		"replace example.com/dependency => " + filepath.ToSlash(dependency),
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("build module missing %q:\n%s", want, source)
		}
	}
	assertMode(t, filepath.Join(buildDir, "go.mod"), 0o600)
}

func TestLocalModuleVersion(t *testing.T) {
	for _, test := range []struct {
		path string
		want string
	}{
		{path: "example.com/device", want: "v0.0.0"},
		{path: "example.com/device/v1", want: "v0.0.0"},
		{path: "example.com/device/v2", want: "v2.0.0"},
		{path: "example.com/device/v12", want: "v12.0.0"},
		{path: "example.com/device/version2", want: "v0.0.0"},
	} {
		t.Run(test.path, func(t *testing.T) {
			if got := localModuleVersion(test.path); got != test.want {
				t.Fatalf("localModuleVersion(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestRTTDoesNotBuildOrCreateFirmwareState(t *testing.T) {
	moduleDir, logPath := fakeToolEnvironment(t, "pyocd")
	request := Request{
		InstanceName: "test.node",
		Manifest:     firmware.Manifest{Package: "example.com/acme/fw"},
		Chip:         inventory.Chip{PyocdTarget: "probe-target"},
		ModuleDir:    moduleDir,
	}
	if err := RTT(context.Background(), request); err != nil {
		t.Fatalf("RTT: %v", err)
	}
	if lines := readToolLog(t, logPath); !reflect.DeepEqual(lines, []string{"pyocd rtt -t probe-target"}) {
		t.Fatalf("tool calls = %v", lines)
	}
	if _, err := os.Stat(filepath.Join(moduleDir, buildStateDir)); !os.IsNotExist(err) {
		t.Fatalf("standalone RTT created firmware state: %v", err)
	}
}

func fakeToolEnvironment(t *testing.T, tools ...string) (moduleDir, logPath string) {
	t.Helper()
	moduleDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte("module example.com/site\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	logPath = filepath.Join(t.TempDir(), "tools.log")
	script := `#!/bin/sh
name=$(basename "$0")
printf '%s %s\n' "$name" "$*" >> "$FWBUILD_TEST_LOG"
case "$name" in
  tinygo) printf 'elf' > image.elf ;;
  arm-none-eabi-objdump) printf 'disassembly\n' ;;
esac
`
	for _, tool := range tools {
		if err := os.WriteFile(filepath.Join(binDir, tool), []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("FWBUILD_TEST_LOG", logPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return moduleDir, logPath
}

func readToolLog(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %#o, want %#o", path, got, want)
	}
}
