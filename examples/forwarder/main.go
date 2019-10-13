package main

import (
	"fmt"

	"github.com/Krajiyah/ble-sdk/pkg/forwarder"
)

const (
	serviceName = "My BLE Forwarder"
)

var (
	// BLESecret is a compile time var (ldflag)
	BLESecret string
	// BLEForwarderAddr is a compile time var (ldflag)
	BLEForwarderAddr string
	// BLEServerAddr is a compile time var (ldflag)
	BLEServerAddr string
)

func main() {
	if BLESecret == "" || BLEForwarderAddr == "" || BLEServerAddr == "" {
		fmt.Println("please compile this with BLESecret, BLEForwarderAddr, and BLEServerAddr as ldflag")
		return
	}
	forward, err := forwarder.NewBLEForwarder(serviceName, BLEForwarderAddr, BLESecret, BLEServerAddr)
	if err != nil {
		fmt.Println("Ooops! Something went wrong with setting up ble client: " + err.Error())
	}
	forward.Run()
}
