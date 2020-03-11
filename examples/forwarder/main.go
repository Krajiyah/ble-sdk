package main

import (
	"fmt"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/forwarder"
	"github.com/Krajiyah/ble-sdk/pkg/models"
)

var (
	// Name is a compile time var (ldflag)
	Name string
	// BLESecret is a compile time var (ldflag)
	BLESecret string
	// BLEForwarderAddr is a compile time var (ldflag)
	BLEForwarderAddr string
	// BLEServerAddr is a compile time var (ldflag)
	BLEServerAddr string
)

type myListener struct{}

func (l myListener) OnClientStateMapChanged(m map[string]models.BLEClientState) {}
func (l myListener) OnClientLog(r models.ClientLogRequest)                      {}
func (l myListener) OnInternalError(err error)                                  {}
func (l myListener) OnConnected(addr string)                                    {}
func (l myListener) OnDisconnected()                                            {}
func (l myListener) OnTimeSync()                                                {}

func main() {
	if Name == "" || BLESecret == "" || BLEForwarderAddr == "" || BLEServerAddr == "" {
		fmt.Println("please compile this with BLESecret, BLEForwarderAddr, and BLEServerAddr as ldflag")
		return
	}
	forward, err := forwarder.NewBLEForwarder(Name, BLEForwarderAddr, BLESecret, BLEServerAddr, time.Minute, myListener{})
	if err != nil {
		fmt.Println("Ooops! Something went wrong with setting up ble client: " + err.Error())
	}
	forward.Run()
}
