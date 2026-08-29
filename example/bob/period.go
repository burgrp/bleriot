package main

import "sync/atomic"

func writePeriod(period *atomic.Int32, value int32, null bool) {
	if null {
		value = 0
	}
	period.Store(value)
}
