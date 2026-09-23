package main

import "testing"

func TestRedLEDOn(t *testing.T) {
	tests := []struct {
		name              string
		online, heartbeat bool
		want              bool
	}{
		{name: "online heartbeat off", online: true},
		{name: "online heartbeat on", online: true, heartbeat: true},
		{name: "offline heartbeat off"},
		{name: "offline heartbeat on", heartbeat: true, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := redLEDOn(test.online, test.heartbeat); got != test.want {
				t.Fatalf("redLEDOn(%v, %v) = %v, want %v", test.online, test.heartbeat, got, test.want)
			}
		})
	}
}
