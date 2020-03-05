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
	// Name is a compile time var (ldflag)
	Name string
	// BLESecret is a compile time var (ldflag)
	BLESecret string
	// BLEClientAddr is a compile time var (ldflag)
	BLEClientAddr string
	// BLEServerAddr is a compile time var (ldflag)
	BLEServerAddr string
)

type exampleListener struct {
	c *client.BLEClient
}

func (l *exampleListener) OnConnected(addr string) {
	fmt.Printf("Client connected to server (%s)", addr)
	for {
		time.Sleep(time.Second * 1)
		fmt.Println("Attempting to write to extra write char...")
		err := l.c.WriteValue(exampleWriteCharUUID, []byte("Hello Server! ~ from Client"))
		if err != nil {
			fmt.Println("Could not write to extra write char :( " + err.Error())
		} else {
			fmt.Println("Wrote to extra write char :)")
		}
	}
}

func (l *exampleListener) OnDisconnected() {
	fmt.Println("Client has disconnected from server.")
}

func (l *exampleListener) OnTimeSync() {
	fmt.Println("Client has syncronized time with server.")
}

func (l *exampleListener) OnInternalError(err error) {
	fmt.Println("Internal Error: " + err.Error())
}

func main() {
	if Name == "" || BLESecret == "" || BLEClientAddr == "" || BLEServerAddr == "" {
		fmt.Println("please compile this with BLESecret, BLEClientAddr, and BLEServerAddr as ldflag")
		return
	}
	var clien *client.BLEClient
	clien, err := client.NewBLEClient(Name, BLEClientAddr, BLESecret, BLEServerAddr, &exampleListener{c: clien})
	if err != nil {
		fmt.Println("Ooops! Something went wrong with setting up ble client: " + err.Error())
	}
	clien.Run()
}
