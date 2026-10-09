// Package fwbuild builds and flashes importable BleRiot firmware packages.
// It turns board-owned, typed profiles into structured tool invocations; it
// never executes board- or deployment-provided shell fragments.
package fwbuild

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/burgrp/bleriot/lib/shared/firmware"
	"github.com/burgrp/bleriot/lib/shared/inventory"
)

const (
	buildStateDir = ".bleriot"
	imageName     = "image.elf"
)

// BuildOptions selects optional stages after the mandatory TinyGo build.
// Stages always run in the order disassembly, flash, then RTT.
type BuildOptions struct {
	Disassembly bool
	Flash       bool
	RTT         bool
}

type buildStep uint8

const (
	stepBuild buildStep = iota
	stepDisassembly
	stepFlash
	stepRTT
)

// Request contains the deployment data and board profile needed for one
// firmware operation. Source is a generated package main containing the
// instance's provisioning and configuration.
type Request struct {
	InstanceName string
	Source       string
	Manifest     firmware.Manifest
	Chip         inventory.Chip
	ModuleDir    string
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
}

// Build generates and compiles one node firmware, then performs the optional
// stages in deterministic order. Generated source and artifacts are kept in a
// private, per-instance directory under .bleriot so Go resolves the consumer's
// pinned module graph and replacements.
func Build(ctx context.Context, request Request, options BuildOptions) error {
	if err := request.Manifest.Validate(); err != nil {
		return err
	}
	if err := preflightBuild(request, options); err != nil {
		return err
	}
	moduleRoot, err := findModuleRoot(request.ModuleDir)
	if err != nil {
		return err
	}
	buildDir := filepath.Join(moduleRoot, buildStateDir, "firmware", safeInstanceName(request.InstanceName))
	if err := os.MkdirAll(buildDir, 0o700); err != nil {
		return fmt.Errorf("creating firmware build directory: %w", err)
	}
	for _, dir := range []string{
		filepath.Join(moduleRoot, buildStateDir),
		filepath.Join(moduleRoot, buildStateDir, "firmware"),
		buildDir,
	} {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("securing firmware directory: %w", err)
		}
	}
	if err := writeBuildModule(ctx, moduleRoot, buildDir); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(buildDir, "main.go"), []byte(request.Source), 0o600); err != nil {
		return fmt.Errorf("writing generated firmware main: %w", err)
	}
	if err := os.Chmod(filepath.Join(buildDir, "main.go"), 0o600); err != nil {
		return fmt.Errorf("securing generated firmware main: %w", err)
	}

	runner := commandRunner{
		ctx:    ctx,
		dir:    buildDir,
		stdin:  request.Stdin,
		stdout: request.Stdout,
		stderr: request.Stderr,
	}
	for _, step := range buildSteps(options) {
		switch step {
		case stepBuild:
			err = runner.build(request)
		case stepDisassembly:
			err = runner.disassemble()
		case stepFlash:
			if err = runner.reclaimProbe(request.Manifest.PyOCD); err == nil {
				err = runner.command("pyocd", pyocdLoadArgs(request.Chip, request.Manifest.PyOCD)...)
			}
		case stepRTT:
			err = runner.command("pyocd", pyocdRTTArgs(request.Chip, request.Manifest.PyOCD)...)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// RTT opens an RTT session without generating or rebuilding firmware.
func RTT(ctx context.Context, request Request) error {
	runner, err := prepareDirectTool(ctx, request, "pyocd")
	if err != nil {
		return err
	}
	if request.Chip.PyocdTarget == "" {
		return fmt.Errorf("firmware chip has no pyOCD target")
	}
	return runner.command("pyocd", pyocdRTTArgs(request.Chip, request.Manifest.PyOCD)...)
}

// GDB starts a pyOCD GDB server without generating or rebuilding firmware.
func GDB(ctx context.Context, request Request) error {
	runner, err := prepareDirectTool(ctx, request, "pyocd")
	if err != nil {
		return err
	}
	if request.Chip.PyocdTarget == "" {
		return fmt.Errorf("firmware chip has no pyOCD target")
	}
	return runner.command("pyocd", pyocdGDBArgs(request.Chip, request.Manifest.PyOCD)...)
}

// InstallPack installs the node chip's CMSIS pack without generating or
// rebuilding firmware.
func InstallPack(ctx context.Context, request Request) error {
	runner, err := prepareDirectTool(ctx, request, "pyocd")
	if err != nil {
		return err
	}
	if request.Chip.CmsisPack == "" {
		return fmt.Errorf("firmware chip has no CMSIS pack")
	}
	return runner.command("pyocd", "pack", "install", request.Chip.CmsisPack)
}

func (runner commandRunner) build(request Request) error {
	if err := runner.commandEnv(buildModuleEnv(), "tinygo", tinyGoBuildArgs(request.Chip, request.Manifest.TinyGo)...); err != nil {
		return err
	}
	if err := os.Chmod(filepath.Join(runner.dir, imageName), 0o600); err != nil {
		return fmt.Errorf("securing firmware image: %w", err)
	}
	if request.Manifest.TinyGo.SizeReport == firmware.SizeReportHTML {
		if err := os.Chmod(filepath.Join(runner.dir, "size-report.html"), 0o600); err != nil {
			return fmt.Errorf("securing firmware size report: %w", err)
		}
	}
	if _, err := os.Stat(filepath.Join(runner.dir, "go.sum")); err == nil {
		if err := os.Chmod(filepath.Join(runner.dir, "go.sum"), 0o600); err != nil {
			return fmt.Errorf("securing firmware module sums: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking firmware module sums: %w", err)
	}
	return nil
}

type commandRunner struct {
	ctx    context.Context
	dir    string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (runner commandRunner) command(name string, args ...string) error {
	return runner.commandEnv(os.Environ(), name, args...)
}

func (runner commandRunner) commandEnv(env []string, name string, args ...string) error {
	slog.Debug("running firmware tool", "dir", runner.dir, "command", name, "args", args)
	cmd := exec.CommandContext(runner.ctx, name, args...)
	cmd.Dir = runner.dir
	cmd.Env = env
	cmd.Stdin = runner.stdin
	cmd.Stdout = runner.stdout
	cmd.Stderr = runner.stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func (runner commandRunner) reclaimProbe(profile firmware.PyOCDProfile) error {
	if !profile.Reclaim {
		return nil
	}
	cmd := exec.CommandContext(runner.ctx, "pkill", "pyocd")
	cmd.Stdout = runner.stdout
	cmd.Stderr = runner.stderr
	if err := cmd.Run(); err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
			return fmt.Errorf("pkill pyocd: %w", err)
		}
	}
	if delay := profile.ReclaimDelayMilliseconds; delay != 0 {
		timer := time.NewTimer(time.Duration(delay) * time.Millisecond)
		defer timer.Stop()
		select {
		case <-runner.ctx.Done():
			return runner.ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

func (runner commandRunner) disassemble() error {
	path := filepath.Join(runner.dir, "disassembly.txt")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("creating disassembly: %w", err)
	}

	cmd := exec.CommandContext(runner.ctx, "arm-none-eabi-objdump", "-dS", imageName)
	cmd.Dir = runner.dir
	cmd.Stdin = runner.stdin
	cmd.Stdout = file
	cmd.Stderr = runner.stderr
	if err := cmd.Run(); err != nil {
		file.Close()
		return fmt.Errorf("arm-none-eabi-objdump: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("closing disassembly: %w", err)
	}
	return nil
}

func tinyGoBuildArgs(chip inventory.Chip, profile firmware.TinyGoProfile) []string {
	args := []string{"build"}
	if profile.StackSizeBytes != 0 {
		args = append(args, "--stack-size", strconv.FormatUint(uint64(profile.StackSizeBytes), 10)+"b")
	}
	if profile.SizeReport != "" {
		args = append(args, "--size", string(profile.SizeReport))
	}
	if profile.Serial != "" {
		args = append(args, "--serial", string(profile.Serial))
	}
	if profile.Scheduler != "" {
		args = append(args, "--scheduler", string(profile.Scheduler))
	}
	if profile.PrintAllocs {
		args = append(args, "-print-allocs=.")
	}
	if profile.GarbageCollector != "" {
		args = append(args, "--gc", string(profile.GarbageCollector))
	}
	args = append(args, "--target", chip.TinygoTarget, "-o", imageName, ".")
	return args
}

func pyocdLoadArgs(chip inventory.Chip, profile firmware.PyOCDProfile) []string {
	args := []string{"load", "-t", chip.PyocdTarget}
	args = appendPyocdOptions(args, profile.FrequencyHz, profile.LoadMode)
	return append(args, imageName)
}

func pyocdRTTArgs(chip inventory.Chip, profile firmware.PyOCDProfile) []string {
	args := []string{"rtt", "-t", chip.PyocdTarget}
	return appendPyocdOptions(args, profile.FrequencyHz, profile.RTTMode)
}

func pyocdGDBArgs(chip inventory.Chip, profile firmware.PyOCDProfile) []string {
	args := []string{"gdb", "--target", chip.PyocdTarget}
	args = appendPyocdOptions(args, profile.FrequencyHz, profile.GDBMode)
	return append(args, "--semihosting", "--persist")
}

func appendPyocdOptions(args []string, frequencyHz uint32, mode firmware.ConnectMode) []string {
	if frequencyHz != 0 {
		args = append(args, "-f", strconv.FormatUint(uint64(frequencyHz), 10))
	}
	if mode != "" {
		args = append(args, "-M", string(mode))
	}
	return args
}

func buildModuleEnv() []string {
	env := os.Environ()
	workFound := false
	for index, value := range env {
		if strings.HasPrefix(value, "GOFLAGS=") {
			env[index] = "GOFLAGS=" + buildModuleGoFlags(strings.TrimPrefix(value, "GOFLAGS="))
		}
		if strings.HasPrefix(value, "GOWORK=") {
			env[index] = "GOWORK=off"
			workFound = true
		}
	}
	if !containsEnv(env, "GOFLAGS=") {
		env = append(env, "GOFLAGS=-mod=mod")
	}
	if !workFound {
		env = append(env, "GOWORK=off")
	}
	return env
}

func buildModuleGoFlags(value string) string {
	fields := strings.Fields(value)
	result := make([]string, 0, len(fields)+1)
	for index := 0; index < len(fields); index++ {
		if fields[index] == "-mod" {
			index++
			continue
		}
		if strings.HasPrefix(fields[index], "-mod=") {
			continue
		}
		result = append(result, fields[index])
	}
	return strings.Join(append(result, "-mod=mod"), " ")
}

func containsEnv(env []string, prefix string) bool {
	for _, value := range env {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

type selectedModule struct {
	Path      string
	Version   string
	Main      bool
	Dir       string
	GoVersion string
	Replace   *selectedModule
}

func writeBuildModule(ctx context.Context, moduleRoot, buildDir string) error {
	modules, err := listSelectedModules(ctx, moduleRoot)
	if err != nil {
		return err
	}
	source, err := renderBuildModule(modules, moduleRoot)
	if err != nil {
		return err
	}
	path := filepath.Join(buildDir, "go.mod")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		return fmt.Errorf("writing firmware build module: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("securing firmware build module: %w", err)
	}
	if err := os.Remove(filepath.Join(buildDir, "go.sum")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("resetting firmware module sums: %w", err)
	}
	return nil
}

func listSelectedModules(ctx context.Context, moduleRoot string) ([]selectedModule, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-mod=readonly", "-m", "-json", "all")
	cmd.Dir = moduleRoot
	cmd.Env = os.Environ()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return nil, fmt.Errorf("listing selected modules: %s", message)
		}
		return nil, fmt.Errorf("listing selected modules: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(out))
	var modules []selectedModule
	for {
		var module selectedModule
		if err := decoder.Decode(&module); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decoding selected modules: %w", err)
		}
		modules = append(modules, module)
	}
	return modules, nil
}

func renderBuildModule(modules []selectedModule, moduleRoot string) (string, error) {
	if len(modules) == 0 {
		return "", fmt.Errorf("selected module graph is empty")
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Path < modules[j].Path })

	goVersion := "1.25.0"
	for _, module := range modules {
		if module.Main && sameDirectory(module.Dir, moduleRoot) && module.GoVersion != "" {
			goVersion = module.GoVersion
			break
		}
	}

	var requires strings.Builder
	var replaces strings.Builder
	for _, module := range modules {
		if module.Path == "" {
			continue
		}
		version := module.Version
		if version == "" {
			version = localModuleVersion(module.Path)
		}
		fmt.Fprintf(&requires, "\t%s %s\n", module.Path, version)

		switch {
		case module.Main && module.Dir != "":
			fmt.Fprintf(&replaces, "replace %s => %s\n", module.Path, filepath.ToSlash(module.Dir))
		case module.Replace != nil && module.Replace.Version != "":
			fmt.Fprintf(&replaces, "replace %s => %s %s\n", module.Path, module.Replace.Path, module.Replace.Version)
		case module.Replace != nil:
			target := module.Replace.Dir
			if target == "" {
				target = module.Replace.Path
			}
			fmt.Fprintf(&replaces, "replace %s => %s\n", module.Path, filepath.ToSlash(target))
		}
	}

	var source strings.Builder
	source.WriteString("module bleriot.local/firmware\n\n")
	fmt.Fprintf(&source, "go %s\n\n", goVersion)
	source.WriteString("require (\n")
	source.WriteString(requires.String())
	source.WriteString(")\n")
	if replaces.Len() != 0 {
		source.WriteString("\n")
		source.WriteString(replaces.String())
	}
	return source.String(), nil
}

func localModuleVersion(path string) string {
	majorSuffix := path[strings.LastIndex(path, "/")+1:]
	if len(majorSuffix) < 2 || majorSuffix[0] != 'v' {
		return "v0.0.0"
	}
	major, err := strconv.Atoi(majorSuffix[1:])
	if err != nil || major < 2 {
		return "v0.0.0"
	}
	return fmt.Sprintf("v%d.0.0", major)
}

func sameDirectory(left, right string) bool {
	leftPath, leftErr := filepath.EvalSymlinks(left)
	rightPath, rightErr := filepath.EvalSymlinks(right)
	if leftErr == nil && rightErr == nil {
		return filepath.Clean(leftPath) == filepath.Clean(rightPath)
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func findModuleRoot(start string) (string, error) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("determining working directory: %w", err)
		}
	}
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolving working directory: %w", err)
	}
	for {
		info, statErr := os.Stat(filepath.Join(dir, "go.mod"))
		if statErr == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found at or above %s", start)
		}
		dir = parent
	}
}

func safeInstanceName(name string) string {
	if name == "" {
		return "default"
	}
	var builder strings.Builder
	for index := 0; index < len(name); index++ {
		character := name[index]
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '.' || character == '-' || character == '_' {
			builder.WriteByte(character)
		} else {
			fmt.Fprintf(&builder, "%%%02X", character)
		}
	}
	result := builder.String()
	if result == "." {
		return "%2E"
	}
	if result == ".." {
		return "%2E%2E"
	}
	return result
}

func buildSteps(options BuildOptions) []buildStep {
	steps := []buildStep{stepBuild}
	if options.Disassembly {
		steps = append(steps, stepDisassembly)
	}
	if options.Flash {
		steps = append(steps, stepFlash)
	}
	if options.RTT {
		steps = append(steps, stepRTT)
	}
	return steps
}

func preflightBuild(request Request, options BuildOptions) error {
	if request.Source == "" {
		return fmt.Errorf("generated firmware source is required")
	}
	if request.Chip.TinygoTarget == "" {
		return fmt.Errorf("firmware chip has no TinyGo target")
	}
	if err := requireTool("go"); err != nil {
		return err
	}
	if err := requireTool("tinygo"); err != nil {
		return err
	}
	if options.Disassembly {
		if err := requireTool("arm-none-eabi-objdump"); err != nil {
			return err
		}
	}
	if options.Flash || options.RTT {
		if request.Chip.PyocdTarget == "" {
			return fmt.Errorf("firmware chip has no pyOCD target")
		}
		if err := requireTool("pyocd"); err != nil {
			return err
		}
	}
	if options.Flash && request.Manifest.PyOCD.Reclaim {
		if err := requireTool("pkill"); err != nil {
			return err
		}
	}
	return nil
}

func prepareDirectTool(ctx context.Context, request Request, tool string) (commandRunner, error) {
	if err := request.Manifest.Validate(); err != nil {
		return commandRunner{}, err
	}
	if err := requireTool(tool); err != nil {
		return commandRunner{}, err
	}
	dir, err := findModuleRoot(request.ModuleDir)
	if err != nil {
		return commandRunner{}, err
	}
	return commandRunner{
		ctx:    ctx,
		dir:    dir,
		stdin:  request.Stdin,
		stdout: request.Stdout,
		stderr: request.Stderr,
	}, nil
}

func requireTool(name string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("required tool %q not found: %w", name, err)
	}
	return nil
}
