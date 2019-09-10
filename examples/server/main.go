package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/server"
)

const (
	serviceName          = "My BLE Server"
	exampleReadCharUUID  = "10010000-0001-1000-8000-00805F9B34FB"
	exampleWriteCharUUID = "10010000-0001-1000-8000-00805F9B34FB"
)

// BLESecret is a compile time var (ldflag)
var BLESecret string

func main() {
	if BLESecret == "" {
		fmt.Errorf("please compile this with BLESecret as ldflag")
		return
	}
	moreReadChars := []*server.BLEReadCharacteristic{
		&server.BLEReadCharacteristic{exampleReadCharUUID, func(context.Context) ([]byte, error) {
			return []byte("Hello World!"), nil
		}, func() {
			for {
				time.Sleep(time.Second * 1)
				fmt.Println("Saying hello to myself (server) in background...")
			}
		}},
	}
	moreWriteChars := []*server.BLEWriteCharacteristic{
		&server.BLEWriteCharacteristic{exampleWriteCharUUID, func(addr string, data []byte, err error) {
			if err != nil {
				fmt.Println("Something went wrong with client writing data to server: " + err.Error())
			} else {
				fmt.Println("Client wrote some data to server: " + string(data))
			}
		}, func() {
			for {
				time.Sleep(time.Second * 1)
				fmt.Println("waiting for clients to tell me (server) something in background...")
			}
		}},
	}
	l := server.BLEServerStatusListener{
		func(s models.BLEServerStatus, err error) {
			fmt.Println(fmt.Sprintf("Server status changed: %s, Error: %s", s, err))
		},
		func(m map[string]models.BLEClientState) {
			fmt.Println(fmt.Sprintf("Client status changed: %+v", m))
		},
		func(r models.ClientLogRequest) {
			fmt.Println(fmt.Sprintf("Client pushed log entry: %+v", r))
		},
		func(err error) {
			fmt.Println(fmt.Sprintf("There was an error in handling a read or write operations from a characteristic: %s", err))
		},
	}
	serv, err := server.NewBLEServer(serviceName, BLESecret, l, moreReadChars, moreWriteChars)
	if err != nil {
		fmt.Println("Ooops! Something went wrong with setting up ble server: " + err.Error())
	}
	err = serv.Run()
	if err != nil {
		fmt.Println("Ooops! Something went wrong with running the ble server: " + err.Error())
	}
}
