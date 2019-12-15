package server

import (
	"context"
	"strconv"
	"time"

	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/currantlabs/ble"
	"github.com/currantlabs/ble/linux"
)

const (
	// PollingInterval is the amount of time used for re-doing status update work
	PollingInterval = time.Second * 2
)

// BLEServerInt is a interface used to abstract BLEServer
type BLEServerInt interface {
	Run() error
}

// BLEServer is the struct used for instantiating a ble server
type BLEServer struct {
	name             string
	secret           string
	status           BLEServerStatus
	clientStateMap   map[string]BLEClientState
	packetAggregator util.PacketAggregator
	listener         BLEServerStatusListener
}

// NewBLEServer creates a new BLEService
func NewBLEServer(name string, secret string, listener BLEServerStatusListener,
	moreReadChars []*BLEReadCharacteristic, moreWriteChars []*BLEWriteCharacteristic) (*BLEServer, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	return NewBLEServerSharedDevice(d, name, secret, listener, moreReadChars, moreWriteChars)
}

func newBLEServer(name string, secret string, listener BLEServerStatusListener) *BLEServer {
	return &BLEServer{
		name, secret, Running,
		map[string]BLEClientState{}, util.NewPacketAggregator(), listener,
	}
}

// NewBLEServerSharedDevice creates a new BLEService using shared ble device
func NewBLEServerSharedDevice(device ble.Device, name string, secret string, listener BLEServerStatusListener,
	moreReadChars []*BLEReadCharacteristic, moreWriteChars []*BLEWriteCharacteristic) (*BLEServer, error) {
	server := newBLEServer(name, secret, listener)
	ble.SetDefaultDevice(device)
	err := ble.AddService(getService(server, moreReadChars, moreWriteChars))
	if err != nil {
		return nil, err
	}
	return server, nil
}

// Run is the method called by BLEService struct for when the service is ready to listen to ble clients
func (server *BLEServer) Run() error {
	ctx := util.MakeINFContext()
	err := ble.AdvertiseNameAndServices(ctx, server.name, ble.MustParse(util.MainServiceUUID))
	server.setStatus(Crashed, err)
	for addr := range server.clientStateMap {
		server.setClientState(addr, BLEClientState{Status: Disconnected, RssiMap: map[string]map[string]int{}})
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
	service := ble.NewService(ble.MustParse(util.MainServiceUUID))
	readChars := []*BLEReadCharacteristic{
		newTimeSyncChar(server),
	}
	if moreReadChars != nil {
		readChars = append(readChars, moreReadChars...)
		for _, char := range readChars {
			service.AddCharacteristic(constructReadChar(server, char))
		}
	}
	writeChars := []*BLEWriteCharacteristic{
		newClientStatusChar(server),
		newClientLogChar(server),
	}
	if moreWriteChars != nil {
		writeChars = append(writeChars, moreWriteChars...)
		for _, char := range writeChars {
			service.AddCharacteristic(constructWriteChar(server, char))
		}
	}
	return service
}

func newClientStatusChar(server *BLEServer) *BLEWriteCharacteristic {
	lastHeard := map[string]int64{}
	return &BLEWriteCharacteristic{util.ClientStateUUID, func(addr string, data []byte, err error) {
		if err != nil {
			server.listener.OnReadOrWriteError(err)
			return
		}
		lastHeard[addr] = util.UnixTS()
		r, err := GetClientStateRequestFromBytes(data)
		if err != nil {
			server.listener.OnReadOrWriteError(err)
			return
		}
		state := BLEClientState{Status: Connected, RssiMap: r.RssiMap}
		server.setClientState(addr, state)
	}, func() {
		for {
			time.Sleep(PollingInterval)
			diff := util.UnixTS() - int64(PollingInterval.Seconds()*1000)
			for addr := range lastHeard {
				if diff > lastHeard[addr] {
					state := BLEClientState{Status: Disconnected, RssiMap: map[string]map[string]int{}}
					server.setClientState(addr, state)
				}
			}
		}
	}}
}

func newTimeSyncChar(server *BLEServer) *BLEReadCharacteristic {
	return &BLEReadCharacteristic{util.TimeSyncUUID, func(_ string, _ context.Context) ([]byte, error) {
		return []byte(strconv.FormatInt(util.UnixTS(), 10)), nil
	}, func() {}}
}

func newClientLogChar(server *BLEServer) *BLEWriteCharacteristic {
	return &BLEWriteCharacteristic{util.ClientLogUUID, func(addr string, data []byte, err error) {
		if err != nil {
			server.listener.OnReadOrWriteError(err)
			return
		}
		r, err := GetClientLogRequestFromBytes(data)
		if err != nil {
			server.listener.OnReadOrWriteError(err)
			return
		}
		server.listener.OnClientLog(*r)
	}, func() {}}
}
