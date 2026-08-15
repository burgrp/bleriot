// Package mcp2210 is a pure-Go Linux driver for the Microchip MCP2210
// USB-HID-to-SPI bridge. It talks to the chip over a raw HID device
// (/dev/hidraw*) using 64-byte HID reports, with no cgo and no external
// dependencies. It exposes just enough of the MCP2210 command set to configure
// the SPI master and run fixed-size SPI transactions, which is what the BleRiot
// USB radio dongle needs to drive a PAN211x over 3-wire SPI.
package mcp2210

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// USB identifiers of a stock MCP2210.
const (
	VendorID  = 0x04D8
	ProductID = 0x00DE
)

// reportLen is the MCP2210 HID report size. Every command and response is
// exactly this many bytes on the wire.
const reportLen = 64

// defaultCommandTimeout bounds one MCP2210 HID request/response exchange. A
// healthy full-speed USB device responds in milliseconds.
const defaultCommandTimeout = 250 * time.Millisecond

// maxStaleDrain bounds how many mismatched (stale) responses command will
// discard while looking for the one matching its request, before giving up.
const maxStaleDrain = 16

// ErrNotFound is returned by Open when no matching MCP2210 device exists.
var ErrNotFound = errors.New("mcp2210: device not found")

// ErrCommandTimeout is returned when an open MCP2210 stops accepting commands
// or producing responses.
var ErrCommandTimeout = errors.New("mcp2210: command timeout")

// Device is an open connection to a single MCP2210 over its hidraw node.
type Device struct {
	f   io.ReadWriteCloser
	cfg SPIConfig
	// commandHook replaces HID I/O in unit tests.
	commandHook func([reportLen]byte) ([reportLen]byte, error)
	// commandTimeout overrides defaultCommandTimeout in unit tests.
	commandTimeout time.Duration
	// lastTxBytes caches the SPI "bytes per transaction" currently programmed
	// into the chip so Transfer only re-sends the SPI settings when the
	// transaction size changes.
	lastTxBytes int
	// gpioValues mirrors the last GPIO output bitmap written via SetGPIO so each
	// update preserves the other pins.
	gpioValues uint16
}

func (d *Device) issue(req [reportLen]byte) ([reportLen]byte, error) {
	if d.commandHook != nil {
		return d.commandHook(req)
	}
	return d.command(req)
}

// Open connects to an MCP2210.
//
// selector chooses the device:
//   - "" selects the first MCP2210 found.
//   - a path beginning with "/dev/" opens that hidraw node directly.
//   - any other value is matched against the USB serial string of each MCP2210.
func Open(selector string) (*Device, error) {
	devnode, err := resolve(selector)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(devnode, os.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("mcp2210: open %s: %w", devnode, err)
	}
	return &Device{f: f, lastTxBytes: -1}, nil
}

// Close releases the underlying hidraw handle.
func (d *Device) Close() error {
	if d.f == nil {
		return nil
	}
	err := d.f.Close()
	d.f = nil
	return err
}

// command sends one 64-byte request report and returns the 64-byte response.
// The Linux hidraw write contract prepends the report number (0 for the
// MCP2210, which does not use numbered reports); reads return the bare report.
//
// If a previous, aborted session left an unread response queued on the device's
// interrupt-IN endpoint, the first read would return that stale report instead
// of this command's. Since every command yields exactly one response, the wanted
// reply is queued behind any stale ones, so reports whose opcode does not match
// the request are discarded until the matching response arrives (bounded, to
// avoid looping forever on a wedged device).
func (d *Device) command(req [reportLen]byte) ([reportLen]byte, error) {
	var resp [reportLen]byte
	buf := make([]byte, reportLen+1) // [0]=report number, then payload
	copy(buf[1:], req[:])
	timeout := d.commandTimeout
	if timeout <= 0 {
		timeout = defaultCommandTimeout
	}
	deadline := time.Now().Add(timeout)
	for {
		n, err := d.f.Write(buf)
		if isWouldBlock(err) && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
			continue
		}
		if isWouldBlock(err) {
			return resp, ErrCommandTimeout
		}
		if err != nil {
			return resp, fmt.Errorf("mcp2210: write report: %w", err)
		}
		if n != len(buf) {
			return resp, fmt.Errorf("mcp2210: short write: %d bytes", n)
		}
		break
	}
	for stale := 0; ; {
		n, err := d.f.Read(resp[:])
		if isWouldBlock(err) && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
			continue
		}
		if isWouldBlock(err) {
			return resp, ErrCommandTimeout
		}
		if err != nil {
			return resp, fmt.Errorf("mcp2210: read report: %w", err)
		}
		if n != reportLen {
			return resp, fmt.Errorf("mcp2210: short response: %d bytes", n)
		}
		if resp[0] == req[0] {
			return resp, nil
		}
		stale++
		if stale > maxStaleDrain {
			return resp, fmt.Errorf("mcp2210: response opcode 0x%02X for command 0x%02X", resp[0], req[0])
		}
		// Stale response from an earlier aborted command; drop it and read on.
	}
}

func isWouldBlock(err error) bool {
	return errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK)
}

// resolve maps a selector to a /dev/hidraw* device node.
func resolve(selector string) (string, error) {
	if strings.HasPrefix(selector, "/dev/") {
		return selector, nil
	}
	devs, err := enumerate()
	if err != nil {
		return "", err
	}
	for _, dev := range devs {
		if selector == "" || dev.serial == selector {
			return dev.devnode, nil
		}
	}
	if selector == "" {
		return "", ErrNotFound
	}
	return "", fmt.Errorf("mcp2210: no device with serial %q: %w", selector, ErrNotFound)
}

// Discover returns a stable selector for every MCP2210 currently connected, for
// the hub's automatic dongle assignment. Each selector is the device's USB
// serial when it has one — so a reconnecting supervisor re-finds it even if it
// returns on a different hidraw node — and otherwise its /dev/hidraw* path. Each
// selector round-trips through Open. A host with no HID subsystem at all yields
// no devices and no error.
func Discover() ([]string, error) {
	devs, err := enumerate()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(devs))
	for _, dev := range devs {
		if dev.serial != "" {
			out = append(out, dev.serial)
		} else {
			out = append(out, dev.devnode)
		}
	}
	return out, nil
}

// hidDevice is one discovered MCP2210 hidraw node and its USB serial (if any).
type hidDevice struct {
	devnode string
	serial  string
}

// enumerate finds every MCP2210 exposed as a hidraw device via sysfs.
func enumerate() ([]hidDevice, error) {
	const base = "/sys/class/hidraw"
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("mcp2210: list %s: %w", base, err)
	}
	var out []hidDevice
	for _, e := range entries {
		name := e.Name() // e.g. "hidraw0"
		devDir := filepath.Join(base, name, "device")
		if !ueventMatches(filepath.Join(devDir, "uevent"), VendorID, ProductID) {
			continue
		}
		out = append(out, hidDevice{
			devnode: filepath.Join("/dev", name),
			serial:  usbSerial(devDir),
		})
	}
	return out, nil
}

// ueventMatches reports whether a HID uevent file declares the given USB vendor
// and product. The relevant line looks like:
//
//	HID_ID=0003:000004D8:000000DE
func ueventMatches(path string, vendor, product uint16) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		id, ok := strings.CutPrefix(line, "HID_ID=")
		if !ok {
			continue
		}
		fields := strings.Split(id, ":")
		if len(fields) != 3 {
			return false
		}
		var v, p uint64
		if _, err := fmt.Sscanf(fields[1], "%x", &v); err != nil {
			return false
		}
		if _, err := fmt.Sscanf(fields[2], "%x", &p); err != nil {
			return false
		}
		return uint16(v) == vendor && uint16(p) == product
	}
	return false
}

// usbSerial walks up the sysfs tree from a HID device directory to the owning
// USB device and returns its serial string, or "" if none is found.
func usbSerial(devDir string) string {
	dir, err := filepath.EvalSymlinks(devDir)
	if err != nil {
		return ""
	}
	for dir != "/" && dir != "." {
		if data, err := os.ReadFile(filepath.Join(dir, "serial")); err == nil {
			return strings.TrimSpace(string(data))
		}
		dir = filepath.Dir(dir)
	}
	return ""
}
