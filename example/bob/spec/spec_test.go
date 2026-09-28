package spec

import (
	"testing"

	"github.com/burgrp/bleriot/lib/shared/firmware"
	"github.com/burgrp/bleriot/lib/shared/inventory"
)

func TestTypeExportsOneWritableLED(t *testing.T) {
	deviceType := Type()
	if err := deviceType.Validate(); err != nil {
		t.Fatalf("Type.Validate: %v", err)
	}
	registers := deviceType.Registers
	if len(registers) != 2 {
		t.Fatalf("register count = %d, want 2", len(registers))
	}

	led := registers[0]
	if led.Tag != 1 || led.Name != "led" || led.Type != inventory.TypeInt || led.ReadOnly {
		t.Fatalf("LED register = %+v, want writable int tag 1 named led", led)
	}

	gpio := registers[1]
	if gpio.Tag != 3 || gpio.Name != "gpio" || gpio.Type != inventory.TypeInt || !gpio.ReadOnly {
		t.Fatalf("GPIO register = %+v, want read-only int tag 3 named gpio", gpio)
	}

	for _, register := range registers {
		if register.Tag == 2 {
			t.Fatal("retired register tag 2 was reused")
		}
	}
}

func TestFirmwareProfile(t *testing.T) {
	profile := Type().Firmware
	if profile.Package != "github.com/burgrp/bleriot/example/bob" {
		t.Fatalf("firmware package = %q", profile.Package)
	}
	if profile.TinyGo.Scheduler != firmware.SchedulerTasks || profile.TinyGo.StackSizeBytes != 1024 {
		t.Fatalf("TinyGo profile = %+v", profile.TinyGo)
	}
	if profile.PyOCD.RTTMode != firmware.ConnectUnderReset || !profile.PyOCD.Reclaim || profile.PyOCD.ReclaimDelayMilliseconds != 1000 {
		t.Fatalf("pyOCD profile = %+v", profile.PyOCD)
	}
}
