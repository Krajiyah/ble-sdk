package forwarder

import (
	"fmt"
	"github.com/pkg/errors"
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
	ReadRssiMapCharUUID  = "00030000-0005-1000-8000-00805F9B34FB"
	scanInterval         = time.Second * 2
	maxConnectAttempts   = 5
	errNotConnected      = "Forwarder is not connected"
	errInvalidForwardReq = "Invalid forwarding request"
)

// BLEForwarder is a struct used to handle mesh network behaviors for forwarder
type BLEForwarder struct {
	addr             string
	forwardingServer server.BLEServerInt
	forwardingClient client.BLEClientInt
	serverAddr       string
	connectedAddr    string
	toConnectAddr    string
	rssiMap          models.RssiMap
	readCharUUIDChan chan string
	listener         models.BLEForwarderListener
}

func newBLEForwarder(addr, serverAddr string, listener models.BLEForwarderListener) *BLEForwarder {
	return &BLEForwarder{
		addr, nil, nil,
		serverAddr, "", "", models.RssiMap{},
		make(chan string),
		listener,
	}
}

// NewBLEForwarder is a function that creates a new ble forwarder
func NewBLEForwarder(name string, addr string, secret string, serverAddr string, serverListener models.BLEServerStatusListener, listener models.BLEForwarderListener) (*BLEForwarder, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	f := newBLEForwarder(addr, serverAddr, listener)
	readChars := []*server.BLEReadCharacteristic{
		newEndReadForwardChar(f),
		newReadRssiMapChar(f),
	}
	writeChars := []*server.BLEWriteCharacteristic{
		newWriteForwardChar(f),
		newStartReadForwardChar(f),
	}
	serv, err := server.NewBLEServerSharedDevice(d, name, secret, serverListener, readChars, writeChars)
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
	forwarder.rssiMap[forwarder.addr] = map[string]int{}
	go forwarder.scanLoop()
	return forwarder.forwardingServer.Run()
}

func (forwarder *BLEForwarder) scanLoop() {
	mutex := &sync.Mutex{}
	for {
		time.Sleep(scanInterval)
		forwarder.forwardingClient.RawScan(func(a ble.Advertisement) {
			mutex.Lock()
			err := forwarder.onScanned(a)
			if err != nil {
				forwarder.listener.OnError(err)
			}
			mutex.Unlock()
		})
	}
}

func (forwarder *BLEForwarder) onScanned(a ble.Advertisement) error {
	rssi := a.RSSI()
	addr := a.Address().String()
	diff := forwarder.rssiMap.Set(forwarder.addr, addr, rssi)
	if diff {
		if addr != forwarder.serverAddr && isForwarder(a) {
			return forwarder.updateRssiMap(addr)
		}
		return forwarder.refreshShortestPath()
	}
	return nil
}

func wrapError(e1, e2 error) error {
	if e2 == nil {
		return e1
	}
	return errors.Wrap(e1, e2.Error())
}

func (forwarder *BLEForwarder) updateRssiMap(addr string) error {
	err := forwarder.keepTryConnect(addr)
	if err != nil {
		e := forwarder.reconnect()
		return wrapError(err, e)
	}
	data, err := forwarder.forwardingClient.ReadValue(ReadRssiMapCharUUID)
	if err != nil {
		e := forwarder.reconnect()
		return wrapError(err, e)
	}
	rssiMap, err := models.GetRssiMapFromBytes(data)
	if err != nil {
		e := forwarder.reconnect()
		return wrapError(err, e)
	}
	forwarder.rssiMap.Merge(rssiMap)
	return forwarder.reconnect()
}

func (forwarder *BLEForwarder) reconnect() error {
	if forwarder.toConnectAddr == "" {
		return nil
	}
	return forwarder.keepTryConnect(forwarder.toConnectAddr)
}

func (forwarder *BLEForwarder) refreshShortestPath() error {
	path, err := util.ShortestPath(forwarder.rssiMap, forwarder.addr, forwarder.serverAddr)
	if err != nil {
		return err
	}
	if len(path) < 2 {
		return fmt.Errorf("Invalid path to server: %s", path)
	}
	nextHop := path[1]
	if forwarder.toConnectAddr != nextHop {
		forwarder.toConnectAddr = nextHop
		err := forwarder.keepTryConnect(nextHop)
		if err == nil {
			forwarder.listener.OnNextHopChanged(nextHop)
		}
		return err
	}
	return nil
}

func isForwarder(a ble.Advertisement) bool {
	for _, service := range a.Services() {
		if util.UuidEqualStr(service, server.MainServiceUUID) {
			return true
		}
	}
	return false
}

func (forwarder *BLEForwarder) keepTryConnect(addr string) error {
	err := errors.New("")
	attempts := 0
	for err != nil && attempts < maxConnectAttempts {
		err = forwarder.connect(addr)
		attempts++
	}
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

func noop() {}

func newReadRssiMapChar(forwarder *BLEForwarder) *server.BLEReadCharacteristic {
	return &server.BLEReadCharacteristic{ReadRssiMapCharUUID, func(addr string, ctx context.Context) ([]byte, error) {
		return forwarder.rssiMap.Data()
	}, noop}
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
	}, noop}
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
	}, noop}
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
	}, noop}
}
