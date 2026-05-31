package main

import (
	"fmt"
	"os"
)

// this would be shared package, but for the draft, we have it in main

type Register struct {
	Name string
	//...and the rest
}

type Instance struct {
	Name      string
	ID        [12]byte
	Key       [16]byte
	Channel   uint8
	Registers []Register
	Config    any
}

type Inventory []Instance

func Start(inventory Inventory) {

	// this would be implemented as cobra - and is actually what we already have in /cli

	if len(os.Args) < 2 {
		fmt.Println("Usage: hub <command>")
		fmt.Println("Commands:")
		fmt.Println("  hub   - Start the hub and print the inventory")
		fmt.Println("  flash - Flash firmware to a device (not implemented)")
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "hub":
		fmt.Println("Hub starting...")

		fmt.Println("Inventory:")
		for _, instance := range inventory {
			fmt.Printf(" - %s\n", instance.Name)
		}
	case "flash":
		fmt.Println("This would lookup the device in the inventory by ID read by SWD, and flash the firmware to it.")

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
	}

}
