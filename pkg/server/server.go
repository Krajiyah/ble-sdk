package server

import (
	"context"
	"strconv"
	"time"

	"github.com/Krajiyah/ble-sdk/internal"
	. "github.com/Krajiyah/ble-sdk/pkg/client"
	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/currantlabs/ble"
	"github.com/currantlabs/ble/linux"
)

const (
	// MainServiceUUID represents UUID for ble service for all ble characteristics
	MainServiceUUID = "00010000-0001-1000-8000-00805F9B34FB"
	// ClientStateUUID represents UUID for ble characteristic which handles writes from client to server for client state changes
	ClientStateUUID = "00010000-0003-1000-8000-00805F9B34FB"
	// TimeSyncUUID represents UUID for ble clients to time sync with ble server
	TimeSyncUUID = "00010000-0004-1000-8000-00805F9B34FB"
	// ClientLogUUID represents UUID for ble clients to write logs to
	ClientLogUUID = "00010000-0006-1000-8000-00805F9B34FB"
	// PollingInterval is the amount of time used for re-doing status update work
	PollingInterval = time.Second * 2
	inf             = time.Hour * 1000000
)

// BLEServer is the struct used for instantiating a ble server
type BLEServer struct {
	name             string
	secret           string
	status           BLEServerStatus
	clientStateMap   map[string]BLEClientState
	packetAggregator util.PacketAggregator
	listener         BLEServerStatusListener
}

// BLEServerStatusListener is an interface which can be used to implement custom state change listeners for server or for clients
type BLEServerStatusListener interface {
	onServerStatusChanged(BLEServerStatus, error)
	onClientStateMapChanged(map[string]BLEClientState)
	onClientLog(*models.ClientLogRequest)
}

// NewBLEServer creates a new BLEService
func NewBLEServer(name string, secret string, listener BLEServerStatusListener) (*BLEServer, error) {
	server := &BLEServer{name, secret, Running, map[string]BLEClientState{}, util.NewPacketAggregator(), listener}
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	ble.SetDefaultDevice(d)
	service := ble.NewService(ble.MustParse(MainServiceUUID))
	service.AddCharacteristic(newClientStatusChar(server))
	service.AddCharacteristic(newTimeSyncChar(server))
	service.AddCharacteristic(newClientLogChar(server))
	if err := ble.AddService(service); err != nil {
		return nil, err
	}
	return server, nil
}

// Run is the method called by BLEService struct for when the service is ready to listen to ble clients
func (server *BLEServer) Run() error {
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), inf))
	err := ble.AdvertiseNameAndServices(ctx, server.name, ble.MustParse(MainServiceUUID))
	server.setStatus(Crashed, err)
	for addr := range server.clientStateMap {
		server.setClientState(addr, BLEClientState{Status: Disconnected, RssiMap: map[string]int{}})
	}
	return err
}

func (server *BLEServer) setStatus(status BLEServerStatus, err error) {
	server.status = status
	server.listener.onServerStatusChanged(status, err)
}

func (server *BLEServer) setClientState(addr string, state BLEClientState) {
	server.clientStateMap[addr] = state
	server.listener.onClientStateMapChanged(server.clientStateMap)
}

func newClientStatusChar(server *BLEServer) *ble.Characteristic {
	lastHeard := map[string]int64{}
	c := newWriteChar(server, ClientStateUUID, func(addr string, data []byte, err error) {
		if err != nil {
			// TODO: how to handle error?
			return
		}
		lastHeard[addr] = internal.UnixTS()
		r, err := models.GetClientStateRequestFromBytes(data)
		if err != nil {
			// TODO: how to handle error?
			return
		}
		state := BLEClientState{Status: Connected, RssiMap: r.RssiMap}
		server.setClientState(addr, state)
	})
	go func() {
		for {
			time.Sleep(PollingInterval)
			diff := internal.UnixTS() - int64(PollingInterval.Seconds()*1000)
			for addr := range lastHeard {
				if diff > lastHeard[addr] {
					state := BLEClientState{Status: Disconnected, RssiMap: map[string]int{}}
					server.setClientState(addr, state)
				}
			}
		}
	}()
	return c
}

func newTimeSyncChar(server *BLEServer) *ble.Characteristic {
	return newReadChar(server, TimeSyncUUID, func(_ context.Context) ([]byte, error) {
		return []byte(strconv.FormatInt(internal.UnixTS(), 10)), nil
	})
}

func newClientLogChar(server *BLEServer) *ble.Characteristic {
	return newWriteChar(server, ClientLogUUID, func(addr string, data []byte, err error) {
		if err != nil {
			// TODO: how to handle error?
			return
		}
		r, err := models.GetClientLogRequestFromBytes(data)
		if err != nil {
			// TODO: how to handle error?
			return
		}
		server.listener.onClientLog(r)
	})
}
