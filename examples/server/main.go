package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/server"
)

const (
	bluetoothAddress     = "11:11:11:11:11:11"
	exampleReadCharUUID  = "10010000-0001-1000-8000-00805F9B34FB"
	exampleWriteCharUUID = "10010000-0001-1000-8000-00805F9B34FB"
)

// Name is a compile time var (ldflag)
var Name string

// BLESecret is a compile time var (ldflag)
var BLESecret string

type myServerListener struct{}

func (l myServerListener) OnServerStatusChanged(s models.BLEServerStatus, err error) {
	fmt.Println(fmt.Sprintf("Server status changed: %s, Error: %s", s, err))
}
func (l myServerListener) OnClientStateMapChanged(c *models.ConnectionGraph, r *models.RssiMap, m map[string]models.BLEClientState) {
	fmt.Println(fmt.Sprintf("Client status changed: %+v, %+v, %+v", c, r, m))
}
func (l myServerListener) OnClientLog(r models.ClientLogRequest) {
	fmt.Println(fmt.Sprintf("Client pushed log entry: %+v", r))
}
func (l myServerListener) OnInternalError(err error) {
	fmt.Println(fmt.Sprintf("There was an error in handling a read or write operations from a characteristic: %s", err))
}

func main() {
	if BLESecret == "" {
		fmt.Println("please compile this with BLESecret as ldflag")
		return
	}
	moreReadChars := []*server.BLEReadCharacteristic{
		&server.BLEReadCharacteristic{exampleReadCharUUID, func(addr string, ctx context.Context) ([]byte, error) {
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
	fmt.Println("Starting BLE server...")
	_, _, err := server.NewBLEServer(Name, BLESecret, bluetoothAddress, nil, myServerListener{}, moreReadChars, moreWriteChars)
	if err != nil {
		fmt.Println("Ooops! Something went wrong with setting up ble server: " + err.Error())
	}
}
