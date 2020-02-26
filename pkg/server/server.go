package server

import (
	"context"
	"strconv"
	"time"

	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
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
	name            string
	addr            string
	secret          string
	status          BLEServerStatus
	connectionGraph *ConnectionGraph
	rssiMap         *RssiMap
	clientStateMap  map[string]BLEClientState
	buffer          *util.PacketBuffer
	listener        BLEServerStatusListener
}

// NewBLEServer creates a new BLEService
func NewBLEServer(name string, addr string, secret string, listener BLEServerStatusListener,
	moreReadChars []*BLEReadCharacteristic, moreWriteChars []*BLEWriteCharacteristic) (*BLEServer, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	return NewBLEServerSharedDevice(d, name, addr, secret, listener, moreReadChars, moreWriteChars)
}

func newBLEServer(name string, addr string, secret string, listener BLEServerStatusListener) *BLEServer {
	return &BLEServer{
		name, addr, secret, Running,
		NewConnectionGraph(), NewRssiMap(),
		map[string]BLEClientState{}, util.NewPacketBuffer(secret), listener,
	}
}

// NewBLEServerSharedDevice creates a new BLEService using shared ble device
func NewBLEServerSharedDevice(device ble.Device, name string, addr string, secret string, listener BLEServerStatusListener,
	moreReadChars []*BLEReadCharacteristic, moreWriteChars []*BLEWriteCharacteristic) (*BLEServer, error) {
	server := newBLEServer(name, addr, secret, listener)
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
		server.setClientState(addr, BLEClientState{Name: "", Status: Disconnected, ConnectedAddr: "", RssiMap: map[string]map[string]int{}})
	}
	return err
}

// GetRssiMap returns the current state of the network from server perspective
func (server *BLEServer) GetRssiMap() *RssiMap {
	return server.rssiMap
}

func (server *BLEServer) GetConnectionGraph() *ConnectionGraph {
	return server.connectionGraph
}

func (server *BLEServer) setStatus(status BLEServerStatus, err error) {
	server.status = status
	server.listener.OnServerStatusChanged(status, err)
}

func (server *BLEServer) setClientState(addr string, state BLEClientState) {
	server.clientStateMap[addr] = state
	if state.Status == Connected {
		server.connectionGraph.Set(addr, server.addr)
	} else {
		server.connectionGraph.Set(addr, "")
	}
	server.rssiMap.Merge(NewRssiMapFromRaw(state.RssiMap))
	server.listener.OnClientStateMapChanged(server.connectionGraph, server.rssiMap, server.clientStateMap)
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
	return &BLEWriteCharacteristic{util.ClientStateUUID, func(a string, data []byte, err error) {
		if err != nil {
			server.listener.OnReadOrWriteError(err)
			return
		}
		r, err := GetClientStateRequestFromBytes(data)
		lastHeard[r.Addr] = util.UnixTS()
		if err != nil {
			server.listener.OnReadOrWriteError(err)
			return
		}
		state := BLEClientState{Name: r.Name, Status: Connected, ConnectedAddr: r.ConnectedAddr, RssiMap: r.RssiMap}
		server.setClientState(r.Addr, state)
	}, func() {
		for {
			time.Sleep(PollingInterval)
			diff := util.UnixTS() - int64(PollingInterval.Seconds()*1000)
			for addr := range lastHeard {
				if diff > lastHeard[addr] {
					state := BLEClientState{Name: "", Status: Disconnected, ConnectedAddr: "", RssiMap: map[string]map[string]int{}}
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
	return &BLEWriteCharacteristic{util.ClientLogUUID, func(_ string, data []byte, err error) {
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
