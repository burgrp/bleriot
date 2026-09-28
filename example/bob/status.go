package bob

func redLEDOn(online, heartbeat bool) bool {
	return !online && heartbeat
}
