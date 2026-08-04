package cli

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/burgrp/bleriot/lib/shared/config"
)

// Probe is the SWD debug-probe surface the onboarding command needs: read a
// device's MCU unique ID. It is a small interface so the command can be tested
// without hardware.
type Probe interface {
	// ReadUID reads the 12-byte MCU unique ID of the attached device.
	ReadUID(ctx context.Context) ([config.UIDLen]byte, error)
}

// PyOCDProbe drives a device over SWD by shelling out to pyocd, the same tool
// used to flash firmware. It reads the UID from a fixed memory address, an
// MCU/firmware fact supplied by the caller.
type PyOCDProbe struct {
	// Target is the pyocd target name (e.g. "py32f030x8").
	Target string
	// UIDAddr is the memory address of the 12-byte MCU unique ID.
	UIDAddr uint32
	// Logger receives debug logs; nil is allowed.
	Logger *slog.Logger
}

func (p *PyOCDProbe) log() *slog.Logger {
	if p.Logger != nil {
		return p.Logger
	}
	return slog.Default()
}

// ReadUID reads the unique ID by having pyocd dump UIDLen bytes from UIDAddr
// into a temporary file, then reading that file back.
func (p *PyOCDProbe) ReadUID(ctx context.Context) ([config.UIDLen]byte, error) {
	var uid [config.UIDLen]byte

	tmp, err := os.CreateTemp("", "bleriot-uid-*.bin")
	if err != nil {
		return uid, err
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName)

	cmd := fmt.Sprintf("savemem 0x%08X %d %q", p.UIDAddr, config.UIDLen, tmpName)
	if err := p.runCommander(ctx, cmd); err != nil {
		return uid, fmt.Errorf("reading UID: %w", err)
	}

	data, err := os.ReadFile(tmpName)
	if err != nil {
		return uid, err
	}
	if len(data) < config.UIDLen {
		return uid, fmt.Errorf("reading UID: got %d bytes, want %d", len(data), config.UIDLen)
	}
	copy(uid[:], data)
	return uid, nil
}

// runCommander runs a single pyocd commander command non-interactively.
func (p *PyOCDProbe) runCommander(ctx context.Context, command string) error {
	return p.run(ctx, "commander", "-t", p.Target, "-c", command)
}

// run executes pyocd with the given arguments, surfacing its output on failure.
func (p *PyOCDProbe) run(ctx context.Context, args ...string) error {
	p.log().Debug("pyocd", "args", args)
	cmd := exec.CommandContext(ctx, "pyocd", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pyocd %v: %w: %s", args, err, out.String())
	}
	return nil
}
