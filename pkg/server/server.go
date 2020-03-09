package server

import (
	"context"
	"fmt"
	"strconv"
	"time"

	. "github.com/Krajiyah/ble-sdk/pkg/ble"
	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
)

const (
	// PollingInterval is the amount of time used for re-doing status update work
	PollingInterval = time.Second * 2
)

type Server interface {
	GetRssiMap() *RssiMap
	GetConnectionGraph() *ConnectionGraph
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
	conn            Connection
	listener        BLEServerStatusListener
}

func NewBLEServer(name string, addr string, secret string, timeout time.Duration, hasScanner bool, cListener BLEClientListener, sListener BLEServerStatusListener,
	moreReadChars []*BLEReadCharacteristic, moreWriteChars []*BLEWriteCharacteristic) (*BLEServer, Connection, error) {
	server := newBLEServer(name, addr, secret, sListener)
	service := getService(server, moreReadChars, moreWriteChars)
	conn, err := NewRealConnection(addr, secret, timeout, cListener, &ServiceInfo{Service: service, ServiceName: name, UUID: ble.MustParse(util.MainServiceUUID)})
	if err != nil {
		return nil, nil, err
	}
	server.conn = conn
	if hasScanner {
		go server.scanLoop()
	}
	return server, conn, nil
}

func newBLEServer(name string, addr string, secret string, listener BLEServerStatusListener) *BLEServer {
	return &BLEServer{
		name, addr, secret, Running,
		NewConnectionGraph(), NewRssiMap(),
		map[string]BLEClientState{}, util.NewPacketBuffer(secret), nil, listener,
	}
}

func (server *BLEServer) scanLoop() {
	for {
		err := server.conn.Scan(func(ble.Advertisement) {})
		if err != nil {
			fmt.Println("Server had problem scanning: " + err.Error())
		}
	}
}

// GetRssiMap returns the current state of the network from server perspective
func (server *BLEServer) GetRssiMap() *RssiMap {
	if server.conn == nil {
		return server.rssiMap
	}
	rm := NewRssiMapFromRaw(server.conn.GetRssiMap().GetAll())
	rm.Merge(server.rssiMap)
	return rm
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
	server.listener.OnClientStateMapChanged(server.connectionGraph, server.GetRssiMap(), server.clientStateMap)
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
			server.listener.OnInternalError(err)
			return
		}
		r, err := GetClientStateRequestFromBytes(data)
		lastHeard[r.Addr] = util.UnixTS()
		if err != nil {
			server.listener.OnInternalError(err)
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
			server.listener.OnInternalError(err)
			return
		}
		r, err := GetClientLogRequestFromBytes(data)
		if err != nil {
			server.listener.OnInternalError(err)
			return
		}
		server.listener.OnClientLog(*r)
	}, func() {}}
}
