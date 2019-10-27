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
	// StartReadForwardCharUUID represents UUID for ble characteristic which handles forwarding of reads
	StartReadForwardCharUUID = "00030000-0004-1000-8000-00805F9B34FB"
	// EndReadForwardCharUUID represents UUID for ble characteristic which handles forwarding of reads
	EndReadForwardCharUUID = "00030000-0006-1000-8000-00805F9B34FB"
	// ReadRssiMapCharUUID represents UUID for ble characteristic which handles forwarding of reads
	ReadRssiMapCharUUID         = "00030000-0005-1000-8000-00805F9B34FB"
	shortestPathRefreshInterval = time.Second * 5
	scanInterval                = time.Second * 2
	maxConnectAttempts          = 5
	errNotConnected             = "Forwarder is not connected"
	errInvalidForwardReq        = "Invalid forwarding request"
)

// BLEForwarder is a struct used to handle mesh network behaviors for forwarder
type BLEForwarder struct {
	addr             string
	forwardingServer *server.BLEServer
	forwardingClient *client.BLEClient
	serverAddr       string
	connectedAddr    string
	toConnectAddr    string
	rssiMap          models.RssiMap
	readCharUUIDChan chan string
	listener         models.BLEForwarderListener
}

// NewBLEForwarder is a function that creates a new ble forwarder
func NewBLEForwarder(name string, addr string, secret string, serverAddr string, serverListener models.BLEServerStatusListener, listener models.BLEForwarderListener) (*BLEForwarder, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	f := &BLEForwarder{
		addr, nil, nil,
		serverAddr, "", "", models.RssiMap{},
		make(chan string),
		listener,
	}
	serv, err := server.NewBLEServerSharedDevice(d, name, secret, serverListener, []*server.BLEReadCharacteristic{
		newEndReadForwardChar(f),
		newReadRssiMapChar(f),
	}, []*server.BLEWriteCharacteristic{
		newWriteForwardChar(f),
		newStartReadForwardChar(f),
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
	forwarder.scanAndUpdateLoops()
	return forwarder.forwardingServer.Run()
}

func (forwarder *BLEForwarder) scanAndUpdateLoops() {
	forwarder.rssiMap[forwarder.addr] = map[string]int{}
	c := make(chan string)
	connectionMutex := &sync.Mutex{}
	go forwarder.scanLoop(c)
	go forwarder.updateRssiMapLoop(connectionMutex, c)
	go forwarder.refreshShortestPathLoop(connectionMutex)
}

func (forwarder *BLEForwarder) scanLoop(c chan string) {
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
}

func (forwarder *BLEForwarder) updateRssiMapLoop(mutex *sync.Mutex, c chan string) {
	for {
		addr := <-c
		err := forwarder.keepTryConnect(mutex, addr)
		if err != nil {
			forwarder.listener.OnConnectionError(err)
			continue
		}
		data, err := forwarder.forwardingClient.ReadValue(ReadRssiMapCharUUID)
		if err != nil {
			forwarder.listener.OnConnectionError(err)
			continue
		}
		rssiMap, err := models.GetRssiMapFromBytes(data)
		if err != nil {
			forwarder.listener.OnError(err)
			continue
		}
		forwarder.rssiMap.Merge(rssiMap)
		if forwarder.toConnectAddr != "" {
			err := forwarder.keepTryConnect(mutex, forwarder.toConnectAddr)
			if err != nil {
				forwarder.listener.OnConnectionError(err)
			}
		}
	}
}

func (forwarder *BLEForwarder) refreshShortestPathLoop(mutex *sync.Mutex) {
	for {
		time.Sleep(shortestPathRefreshInterval)
		path, err := util.ShortestPathToServer(forwarder.addr, forwarder.serverAddr, forwarder.rssiMap)
		if err == nil && forwarder.toConnectAddr != path[0] {
			forwarder.toConnectAddr = path[0]
			err := forwarder.keepTryConnect(mutex, forwarder.toConnectAddr)
			if err != nil {
				forwarder.listener.OnConnectionError(err)
			}
		}
	}
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

func newReadRssiMapChar(forwarder *BLEForwarder) *server.BLEReadCharacteristic {
	return &server.BLEReadCharacteristic{ReadRssiMapCharUUID, func(addr string, ctx context.Context) ([]byte, error) {
		return forwarder.rssiMap.Data()
	}, func() {}}
}

func newWriteForwardChar(forwarder *BLEForwarder) *server.BLEWriteCharacteristic {
	return &server.BLEWriteCharacteristic{WriteForwardCharUUID, func(addr string, data []byte, err error) {
		if err != nil {
			forwarder.listener.OnReadOrWriteError(err)
			return
		}
		if !forwarder.isConnected() {
			forwarder.listener.OnConnectionError(errors.New(errNotConnected))
			return
		}
		if !forwarder.isConnectedToServer() {
			err := forwarder.forwardingClient.WriteValue(WriteForwardCharUUID, data)
			if err != nil {
				forwarder.listener.OnReadOrWriteError(err)
				return
			}
		} else {
			r, err := models.GetForwarderRequestFromBytes(data)
			if err != nil {
				forwarder.listener.OnError(err)
				return
			}
			if r.IsRead || !r.IsWrite {
				forwarder.listener.OnError(errors.New(errInvalidForwardReq))
				return
			}
			err = forwarder.forwardingClient.WriteValue(r.CharUUID, r.Payload)
			if err != nil {
				forwarder.listener.OnReadOrWriteError(err)
				return
			}
		}
	}, func() {}}
}

func newStartReadForwardChar(forwarder *BLEForwarder) *server.BLEWriteCharacteristic {
	return &server.BLEWriteCharacteristic{StartReadForwardCharUUID, func(addr string, data []byte, err error) {
		if err != nil {
			forwarder.listener.OnReadOrWriteError(err)
			return
		}
		if !forwarder.isConnected() {
			forwarder.listener.OnConnectionError(errors.New(errNotConnected))
			return
		}
		if !forwarder.isConnectedToServer() {
			err := forwarder.forwardingClient.WriteValue(StartReadForwardCharUUID, data)
			if err != nil {
				forwarder.listener.OnReadOrWriteError(err)
				return
			}
		} else {
			r, err := models.GetForwarderRequestFromBytes(data)
			if err != nil {
				forwarder.listener.OnError(err)
				return
			}
			if r.IsWrite || !r.IsRead {
				forwarder.listener.OnError(errors.New(errInvalidForwardReq))
				return
			}
			forwarder.readCharUUIDChan <- r.CharUUID
		}
	}, func() {}}
}

func newEndReadForwardChar(forwarder *BLEForwarder) *server.BLEReadCharacteristic {
	return &server.BLEReadCharacteristic{EndReadForwardCharUUID, func(addr string, ctx context.Context) ([]byte, error) {
		if !forwarder.isConnected() {
			return nil, errors.New(errNotConnected)
		}
		if !forwarder.isConnectedToServer() {
			return forwarder.forwardingClient.ReadValue(EndReadForwardCharUUID)
		}
		c := <-forwarder.readCharUUIDChan
		return forwarder.forwardingClient.ReadValue(c)
	}, func() {}}
}
