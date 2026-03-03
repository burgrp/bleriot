package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"tinygo.org/x/bluetooth"
)

func listAdapters() {
	entries, err := os.ReadDir("/sys/class/bluetooth")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot list adapters:", err)
		os.Exit(1)
	}
	if len(entries) == 0 {
		fmt.Println("No Bluetooth adapters found.")
		return
	}
	fmt.Println("Available adapters:")
	for _, e := range entries {
		fmt.Println(" ", e.Name())
	}
}

type Config struct {
	IgnoreAddresses []string `json:"ignoreAddresses"`
}

func loadConfig(path string) Config {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	json.Unmarshal(data, &cfg)
	return cfg
}

func main() {
	if len(os.Args) < 2 {
		listAdapters()
		os.Exit(0)
	}
	adapterID := os.Args[1]

	cfg := loadConfig("config.json")

	ignored := make(map[string]bool, len(cfg.IgnoreAddresses))
	for _, addr := range cfg.IgnoreAddresses {
		ignored[strings.ToUpper(addr)] = true
	}

	adapter := bluetooth.NewAdapter(adapterID)
	must(adapter.Enable())

	fmt.Println("Scanning…")

	err := adapter.Scan(func(a *bluetooth.Adapter, r bluetooth.ScanResult) {
		if ignored[strings.ToUpper(r.Address.String())] {
			return
		}

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
