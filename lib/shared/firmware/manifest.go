// Package firmware describes how BleRiot builds and flashes a node firmware.
// The board module owns this data; deployments only select a module version and
// provide per-instance provisioning and configuration.
package firmware

import (
	"fmt"
	"path"
	"strings"
	"unicode"
)

// Scheduler is a TinyGo scheduler mode.
type Scheduler string

const (
	SchedulerNone  Scheduler = "none"
	SchedulerTasks Scheduler = "tasks"
)

// GarbageCollector is a TinyGo garbage collector mode.
type GarbageCollector string

const (
	GCLeaking GarbageCollector = "leaking"
)

// Serial is a TinyGo serial output implementation.
type Serial string

const (
	SerialRTT Serial = "rtt"
)

// SizeReport is a TinyGo size-report format.
type SizeReport string

const (
	SizeReportHTML SizeReport = "html"
)

// ConnectMode is a pyOCD connection mode. The empty value uses pyOCD's
// default.
type ConnectMode string

const (
	ConnectAttach     ConnectMode = "attach"
	ConnectUnderReset ConnectMode = "under-reset"
)

// TinyGoProfile contains board-owned TinyGo policy. Zero-valued fields retain
// the tool's defaults.
type TinyGoProfile struct {
	Scheduler        Scheduler
	StackSizeBytes   uint32
	GarbageCollector GarbageCollector
	Serial           Serial
	SizeReport       SizeReport
	PrintAllocs      bool
}

// PyOCDProfile contains board-owned probe policy. FrequencyHz and connection
// modes use pyOCD defaults when zero. Reclaim terminates an existing pyOCD
// process before flashing; ReclaimDelayMilliseconds is the settling delay.
type PyOCDProfile struct {
	FrequencyHz              uint32
	LoadMode                 ConnectMode
	RTTMode                  ConnectMode
	GDBMode                  ConnectMode
	Reclaim                  bool
	ReclaimDelayMilliseconds uint32
}

// Manifest identifies an importable firmware runtime and its build policy. The
// package must export Run(node.Provisioning, Config) when built by TinyGo.
type Manifest struct {
	Package string
	TinyGo  TinyGoProfile
	PyOCD   PyOCDProfile
}

// IsZero reports whether no firmware runtime is associated with a device type.
func (manifest Manifest) IsZero() bool {
	return manifest == (Manifest{})
}

// Validate checks that a manifest can be translated into structured TinyGo and
// pyOCD arguments without accepting arbitrary command fragments.
func (manifest Manifest) Validate() error {
	if manifest.Package == "" {
		return fmt.Errorf("firmware package is required")
	}
	if !validPackagePath(manifest.Package) {
		return fmt.Errorf("invalid firmware package %q", manifest.Package)
	}
	if err := validateEnum("TinyGo scheduler", string(manifest.TinyGo.Scheduler), "", string(SchedulerNone), string(SchedulerTasks)); err != nil {
		return err
	}
	if err := validateEnum("TinyGo garbage collector", string(manifest.TinyGo.GarbageCollector), "", string(GCLeaking)); err != nil {
		return err
	}
	if err := validateEnum("TinyGo serial", string(manifest.TinyGo.Serial), "", string(SerialRTT)); err != nil {
		return err
	}
	if err := validateEnum("TinyGo size report", string(manifest.TinyGo.SizeReport), "", string(SizeReportHTML)); err != nil {
		return err
	}
	if err := validateConnectMode("pyOCD load mode", manifest.PyOCD.LoadMode); err != nil {
		return err
	}
	if err := validateConnectMode("pyOCD RTT mode", manifest.PyOCD.RTTMode); err != nil {
		return err
	}
	if err := validateConnectMode("pyOCD GDB mode", manifest.PyOCD.GDBMode); err != nil {
		return err
	}
	if !manifest.PyOCD.Reclaim && manifest.PyOCD.ReclaimDelayMilliseconds != 0 {
		return fmt.Errorf("pyOCD reclaim delay requires reclaim")
	}
	return nil
}

func validPackagePath(packagePath string) bool {
	if packagePath == "" || path.IsAbs(packagePath) || path.Clean(packagePath) != packagePath {
		return false
	}
	for _, element := range strings.Split(packagePath, "/") {
		if element == "" || element == "." || element == ".." {
			return false
		}
	}
	for _, character := range packagePath {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			continue
		}
		switch character {
		case '/', '-', '.', '_', '~':
		default:
			return false
		}
	}
	return true
}

func validateConnectMode(field string, mode ConnectMode) error {
	return validateEnum(field, string(mode), "", string(ConnectAttach), string(ConnectUnderReset))
}

func validateEnum(field, value string, allowed ...string) error {
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	return fmt.Errorf("invalid %s %q", field, value)
}
