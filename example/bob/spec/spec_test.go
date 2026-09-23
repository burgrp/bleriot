package spec

import (
	"testing"

	"github.com/burgrp/bleriot/lib/shared/inventory"
)

func TestTypeExportsOneWritableLED(t *testing.T) {
	registers := Type().Registers
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
