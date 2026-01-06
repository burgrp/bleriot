package main

import (
	"encoding/hex"
	"fmt"
	"time"

	"tinygo.org/x/bluetooth"
)

func main() {
	adapter := bluetooth.DefaultAdapter
	must(adapter.Enable())

	fmt.Println("Scanning…")

	err := adapter.Scan(func(a *bluetooth.Adapter, r bluetooth.ScanResult) {
		fmt.Println("----")

		fmt.Println("Addr:", r.Address)
		fmt.Println("RSSI:", r.RSSI)

		// if len(r.ServiceData()) > 0 {
		// 	for v := range a.ServiceData() {
		// 		fmt.Printf("Service %s\n", v)
		// 	}
		// }

		// Raw advertising payload bytes (adv + scan response if available)
		if len(r.AdvertisementPayload.Bytes()) > 0 {
			fmt.Println("Raw:", hex.EncodeToString(r.AdvertisementPayload.Bytes()))
		}
	})
	must(err)

	// keep scanning
	select {
	case <-time.After(30 * time.Second):
		fmt.Println("Done")
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
