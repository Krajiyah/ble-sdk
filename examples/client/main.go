package main

import (
	"fmt"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/client"
)

const (
	exampleReadCharUUID  = "10010000-0001-1000-8000-00805F9B34FB"
	exampleWriteCharUUID = "10010000-0001-1000-8000-00805F9B34FB"
)

var (
	// BLESecret is a compile time var (ldflag)
	BLESecret string
	// BLEClientAddr is a compile time var (ldflag)
	BLEClientAddr string
	// BLEServerAddr is a compile time var (ldflag)
	BLEServerAddr string
)

func main() {
	if BLESecret == "" || BLEClientAddr == "" || BLEServerAddr == "" {
		fmt.Println("please compile this with BLESecret, BLEClientAddr, and BLEServerAddr as ldflag")
		return
	}
	var clien *client.BLEClient
	clien, err := client.NewBLEClient(BLEClientAddr, BLESecret, BLEServerAddr, func(addr string, attempts int, rssi int) {
		fmt.Printf("Client connected to server (%s) after %d attempts with rssi %d", addr, attempts, rssi)
		for {
			time.Sleep(time.Second * 1)
			fmt.Println("Attempting to write to extra write char...")
			err := clien.WriteValue(exampleWriteCharUUID, []byte("Hello Server! ~ from Client"))
			if err != nil {
				fmt.Println("Could not write to extra write char :( " + err.Error())
			} else {
				fmt.Println("Wrote to extra write char :)")
			}
		}
	}, func() {
		fmt.Println("Client has disconnected from server.")
	})
	if err != nil {
		fmt.Println("Ooops! Something went wrong with setting up ble client: " + err.Error())
	}
	clien.Run()
}
