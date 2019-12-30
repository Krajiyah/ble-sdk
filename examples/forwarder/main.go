package main

import (
	"fmt"

	"github.com/Krajiyah/ble-sdk/pkg/forwarder"
	"github.com/Krajiyah/ble-sdk/pkg/models"
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

type myServerListener struct{}

func (l myServerListener) OnServerStatusChanged(s models.BLEServerStatus, err error)  {}
func (l myServerListener) OnClientStateMapChanged(m map[string]models.BLEClientState) {}
func (l myServerListener) OnClientLog(r models.ClientLogRequest)                      {}
func (l myServerListener) OnReadOrWriteError(err error)                               {}

type myListener struct{}

func (l myListener) OnConnectionError(err error)                           {}
func (l myListener) OnReadOrWriteError(err error)                          {}
func (l myListener) OnError(err error)                                     {}
func (l myListener) OnClientConnected(addr string, attempts int, rssi int) {}

func main() {
	if BLESecret == "" || BLEForwarderAddr == "" || BLEServerAddr == "" {
		fmt.Println("please compile this with BLESecret, BLEForwarderAddr, and BLEServerAddr as ldflag")
		return
	}
	forward, err := forwarder.NewBLEForwarder(serviceName, BLEForwarderAddr, BLESecret, BLEServerAddr, myServerListener{}, myListener{})
	if err != nil {
		fmt.Println("Ooops! Something went wrong with setting up ble client: " + err.Error())
	}
	forward.Run()
}
