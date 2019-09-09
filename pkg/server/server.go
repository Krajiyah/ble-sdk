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

// BLEServerStatusListener is an struct which can be used to implement custom state change listeners for server or for clients
type BLEServerStatusListener struct {
	OnServerStatusChanged   func(BLEServerStatus, error)
	OnClientStateMapChanged func(map[string]BLEClientState)
	OnClientLog             func(models.ClientLogRequest)
	OnReadOrWriteError      func(error)
}

// NewBLEServer creates a new BLEService
func NewBLEServer(name string, secret string, listener BLEServerStatusListener,
	moreReadChars []*BLEReadCharacteristic, moreWriteChars []*BLEWriteCharacteristic) (*BLEServer, error) {
	server := &BLEServer{name, secret, Running, map[string]BLEClientState{}, util.NewPacketAggregator(), listener}
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	ble.SetDefaultDevice(d)
	if err := ble.AddService(getService(server, moreReadChars, moreWriteChars)); err != nil {
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
	server.listener.OnServerStatusChanged(status, err)
}

func (server *BLEServer) setClientState(addr string, state BLEClientState) {
	server.clientStateMap[addr] = state
	server.listener.OnClientStateMapChanged(server.clientStateMap)
}

func getService(server *BLEServer, moreReadChars []*BLEReadCharacteristic, moreWriteChars []*BLEWriteCharacteristic) *ble.Service {
	service := ble.NewService(ble.MustParse(MainServiceUUID))
	readChars := []*BLEReadCharacteristic{
		newTimeSyncChar(server),
	}
	readChars = append(readChars, moreReadChars...)
	for _, char := range readChars {
		service.AddCharacteristic(constructReadChar(server, char))
	}
	writeChars := []*BLEWriteCharacteristic{
		newClientStatusChar(server),
		newClientLogChar(server),
	}
	writeChars = append(writeChars, moreWriteChars...)
	for _, char := range writeChars {
		service.AddCharacteristic(constructWriteChar(server, char))
	}
	return service
}

func newClientStatusChar(server *BLEServer) *BLEWriteCharacteristic {
	lastHeard := map[string]int64{}
	return &BLEWriteCharacteristic{ClientStateUUID, func(addr string, data []byte, err error) {
		if err != nil {
			server.listener.OnReadOrWriteError(err)
			return
		}
		lastHeard[addr] = internal.UnixTS()
		r, err := models.GetClientStateRequestFromBytes(data)
		if err != nil {
			server.listener.OnReadOrWriteError(err)
			return
		}
		state := BLEClientState{Status: Connected, RssiMap: r.RssiMap}
		server.setClientState(addr, state)
	}, func() {
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
	}}
}

func newTimeSyncChar(server *BLEServer) *BLEReadCharacteristic {
	return &BLEReadCharacteristic{TimeSyncUUID, func(_ context.Context) ([]byte, error) {
		return []byte(strconv.FormatInt(internal.UnixTS(), 10)), nil
	}, func() {}}
}

func newClientLogChar(server *BLEServer) *BLEWriteCharacteristic {
	return &BLEWriteCharacteristic{ClientLogUUID, func(addr string, data []byte, err error) {
		if err != nil {
			server.listener.OnReadOrWriteError(err)
			return
		}
		r, err := models.GetClientLogRequestFromBytes(data)
		if err != nil {
			server.listener.OnReadOrWriteError(err)
			return
		}
		server.listener.OnClientLog(*r)
	}, func() {}}
}
