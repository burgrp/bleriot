package mcpdongle

import (
	"testing"
	"time"

	"github.com/burgrp/tinygo-drivers/pan211x"

	"github.com/burgrp/bleriot/lib/site/engine"
)

func TestReplyGuard(t *testing.T) {
	for _, tc := range []struct {
		name string
		sf   pan211x.SpreadFactor
	}{
		{"S8", pan211x.SpreadFactorS8},
		{"S2", pan211x.SpreadFactorS2},
		{"unknown", pan211x.SpreadFactor(0xff)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ReplyGuard(tc.sf); got != 20*time.Millisecond {
				t.Fatalf("ReplyGuard(%v) = %v, want 20ms", tc.sf, got)
			}
			if got := ReplyGuard(tc.sf); got >= engine.DefaultTimeout {
				t.Fatalf("ReplyGuard(%v) = %v, must be below engine timeout %v", tc.sf, got, engine.DefaultTimeout)
			}
		})
	}
}

func TestLEDTriggerAndStop(t *testing.T) {
	type change struct {
		pin uint8
		on  bool
	}
	changes := make(chan change, 2)
	led := newLED(func(pin uint8, on bool) {
		changes <- change{pin: pin, on: on}
	}, ledGreenPin)

	led.trigger()
	led.stop()
	if got := <-changes; got != (change{pin: ledGreenPin, on: true}) {
		t.Fatalf("trigger change = %+v", got)
	}
	if got := <-changes; got != (change{pin: ledGreenPin, on: false}) {
		t.Fatalf("stop change = %+v", got)
	}
}
