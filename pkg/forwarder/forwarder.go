package forwarder

import (
	"errors"
	"sync"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/client"
	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/server"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/currantlabs/ble"
	"github.com/currantlabs/ble/linux"
	"golang.org/x/net/context"
)

const (
	// WriteForwardCharUUID represents UUID for ble characteristic which handles forwarding of writes
	WriteForwardCharUUID = "00030000-0003-1000-8000-00805F9B34FB"
	// ReadForwardCharUUID represents UUID for ble characteristic which handles forwarding of reads
	ReadForwardCharUUID = "00030000-0004-1000-8000-00805F9B34FB"
	// ReadRssiMapCharUUID represents UUID for ble characteristic which handles forwarding of reads
	ReadRssiMapCharUUID         = "00030000-0005-1000-8000-00805F9B34FB"
	shortestPathRefreshInterval = time.Second * 5
	scanInterval                = time.Second * 2
	maxConnectAttempts          = 5
)

// BLEForwarder is a struct used to handle mesh network behaviors for forwarder
type BLEForwarder struct {
	addr             string
	forwardingServer *server.BLEServer
	forwardingClient *client.BLEClient
	serverAddr       string
	connectedAddr    string
	rssiMap          models.RssiMap
}

// NewBLEForwarder is a function that creates a new ble forwarder
func NewBLEForwarder(name string, addr string, secret string, serverAddr string, listener models.BLEServerStatusListener) (*BLEForwarder, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	f := &BLEForwarder{
		addr, nil, nil,
		serverAddr, "", map[string]map[string]int{},
	}
	serv, err := server.NewBLEServerSharedDevice(d, name, secret, listener, []*server.BLEReadCharacteristic{
		newReadForwardCharHandler(f),
		newReadRssiMapCharHandler(f),
	}, []*server.BLEWriteCharacteristic{
		newWriteForwardCharHandler(f),
	})
	if err != nil {
		return nil, err
	}
	clien, err := client.NewBLEClientSharedDevice(d, addr, secret, serverAddr, func(attempts int, rssi int) {}, func() {})
	if err != nil {
		return nil, err
	}
	f.forwardingServer = serv
	f.forwardingClient = clien
	return f, nil
}

// Run is a method that runs the forwarder forever
func (forwarder *BLEForwarder) Run() error {
	go forwarder.scanLoop()
	return forwarder.forwardingServer.Run()
}

func (forwarder *BLEForwarder) scanLoop() {
	forwarder.rssiMap[forwarder.addr] = map[string]int{}
	c := make(chan string)
	connectionMutex := &sync.Mutex{}
	go func() {
		for {
			time.Sleep(scanInterval)
			forwarder.forwardingClient.RawScan(func(a ble.Advertisement) {
				rssi := a.RSSI()
				addr := a.Address().String()
				forwarder.rssiMap[forwarder.addr][addr] = rssi
				if isForwarderOrServer(a) {
					c <- addr
				}
			})
		}
	}()
	go func() {
		for {
			addr := <-c
			err := forwarder.keepTryConnect(connectionMutex, addr)
			if err != nil {
				// TODO: handle error
				continue
			}
			data, err := forwarder.forwardingClient.ReadValue(ReadRssiMapCharUUID)
			if err != nil {
				// TODO: handle error
				continue
			}
			rssiMap, err := models.GetRssiMapFromBytes(data)
			if err != nil {
				// TODO: handle error
				continue
			}
			forwarder.rssiMap.Merge(rssiMap)
		}
	}()
	go func() {
		nextHopAddrPrev := ""
		for {
			time.Sleep(shortestPathRefreshInterval)
			path, err := util.ShortestPathToServer(forwarder.addr, forwarder.serverAddr, forwarder.rssiMap)
			nextHopAddr := path[0]
			if err == nil && nextHopAddrPrev != nextHopAddr {
				nextHopAddrPrev = nextHopAddr
				err := forwarder.keepTryConnect(connectionMutex, nextHopAddr)
				if err != nil {
					// TODO: handle error
					continue
				}
			}
		}
	}()
}

func isForwarderOrServer(a ble.Advertisement) bool {
	for _, service := range a.Services() {
		if util.UuidEqualStr(service, server.MainServiceUUID) {
			return true
		}
	}
	return false
}

func (forwarder *BLEForwarder) keepTryConnect(mutex *sync.Mutex, addr string) error {
	mutex.Lock()
	err := errors.New("")
	attempts := 0
	for err != nil && attempts < maxConnectAttempts {
		err = forwarder.connect(addr)
		attempts++
	}
	mutex.Unlock()
	return err
}

func (forwarder *BLEForwarder) connect(addr string) error {
	forwarder.connectedAddr = ""
	err := forwarder.forwardingClient.RawConnect(func(a ble.Advertisement) bool {
		return util.AddrEqualAddr(a.Address().String(), addr)
	})
	if err != nil {
		return err
	}
	forwarder.connectedAddr = addr
	return nil
}

func (forwarder *BLEForwarder) isConnected() bool {
	return forwarder.connectedAddr != ""
}

func (forwarder *BLEForwarder) isConnectedToServer() bool {
	return forwarder.connectedAddr == forwarder.serverAddr
}

func newWriteForwardCharHandler(forwarder *BLEForwarder) *server.BLEWriteCharacteristic {
	return &server.BLEWriteCharacteristic{WriteForwardCharUUID, func(addr string, data []byte, err error) {
		if err != nil {
			// TODO: handle error
			return
		}
		if !forwarder.isConnected() {
			// TODO: handle error
			return
		}
		if !forwarder.isConnectedToServer() {
			err := forwarder.forwardingClient.WriteValue(WriteForwardCharUUID, data)
			if err != nil {
				// TODO: handle error
				return
			}
		} else {
			// TODO: unpack data and determine correct server characteristc request
		}
	}, func() {}}
}

func newReadForwardCharHandler(forwarder *BLEForwarder) *server.BLEReadCharacteristic {
	return &server.BLEReadCharacteristic{ReadForwardCharUUID, func(addr string, ctx context.Context) ([]byte, error) {
		if !forwarder.isConnected() {
			return nil, errors.New("Forwarder is not connected")
		}
		if !forwarder.isConnectedToServer() {
			return forwarder.forwardingClient.ReadValue(ReadForwardCharUUID)
		}
		// TODO: unpack data and determine correct server characteristc request
		return nil, nil
	}, func() {}}
}

func newReadRssiMapCharHandler(forwarder *BLEForwarder) *server.BLEReadCharacteristic {
	return &server.BLEReadCharacteristic{ReadRssiMapCharUUID, func(addr string, ctx context.Context) ([]byte, error) {
		return forwarder.rssiMap.Data()
	}, func() {}}
}
