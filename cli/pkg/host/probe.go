package host

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"cli/pkg/page"
)

// Probe is the SWD debug-probe surface the provisioning commands need: read a
// device's MCU unique ID and write a raw image to its provisioning flash page.
// It is a small interface so the commands can be tested without hardware.
type Probe interface {
	// ReadUID reads the 12-byte MCU unique ID of the attached device.
	ReadUID(ctx context.Context) ([page.UIDLen]byte, error)
	// WritePage writes image to the device's provisioning flash page.
	WritePage(ctx context.Context, image []byte) error
}

// PyOCDProbe drives a device over SWD by shelling out to pyocd, the same tool
// used to flash firmware. It reads the UID from a fixed memory address and
// writes the provisioning page to a fixed flash address; both are MCU/firmware
// facts supplied by the caller.
type PyOCDProbe struct {
	// Target is the pyocd target name (e.g. "py32f030x8").
	Target string
	// UIDAddr is the memory address of the 12-byte MCU unique ID.
	UIDAddr uint32
	// PageAddr is the flash address of the provisioning page.
	PageAddr uint32
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
func (p *PyOCDProbe) ReadUID(ctx context.Context) ([page.UIDLen]byte, error) {
	var uid [page.UIDLen]byte

	tmp, err := os.CreateTemp("", "bleriot-uid-*.bin")
	if err != nil {
		return uid, err
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName)

	cmd := fmt.Sprintf("savemem 0x%08X %d %q", p.UIDAddr, page.UIDLen, tmpName)
	if err := p.runCommander(ctx, cmd); err != nil {
		return uid, fmt.Errorf("reading UID: %w", err)
	}

	data, err := os.ReadFile(tmpName)
	if err != nil {
		return uid, err
	}
	if len(data) < page.UIDLen {
		return uid, fmt.Errorf("reading UID: got %d bytes, want %d", len(data), page.UIDLen)
	}
	copy(uid[:], data)
	return uid, nil
}

// WritePage flashes image to the provisioning page address. It writes image to a
// temporary file and loads it at PageAddr.
func (p *PyOCDProbe) WritePage(ctx context.Context, image []byte) error {
	tmp, err := os.CreateTemp("", "bleriot-page-*.bin")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(image); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	args := []string{
		"load",
		"-t", p.Target,
		"--base-address", fmt.Sprintf("0x%08X", p.PageAddr),
		tmpName,
	}
	return p.run(ctx, args...)
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
