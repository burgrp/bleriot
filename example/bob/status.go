package main

func redLEDOn(online, heartbeat bool) bool {
	return !online && heartbeat
}
