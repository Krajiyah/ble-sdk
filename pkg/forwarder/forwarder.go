package forwarder

import (
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"

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
	maxConnectAttempts   = 5
	errNotConnected      = "Forwarder is not connected"
	errInvalidForwardReq = "Invalid forwarding request"
)

// BLEForwarder is a struct used to handle mesh network behaviors for forwarder
type BLEForwarder struct {
	addr              string
	forwardingServer  server.BLEServerInt
	forwardingClient  client.BLEClientInt
	serverAddr        string
	connectedAddr     string
	toConnectAddr     string
	rssiMap           *models.RssiMap
	readCharUUIDMutex *sync.Mutex
	readCharUUID      string
	listener          models.BLEForwarderListener
}

func newBLEForwarder(addr, serverAddr string, listener models.BLEForwarderListener) *BLEForwarder {
	return &BLEForwarder{
		addr, nil, nil,
		serverAddr, "", "", &models.RssiMap{},
		&sync.Mutex{}, "",
		listener,
	}
}

func getChars(f *BLEForwarder) ([]*server.BLEReadCharacteristic, []*server.BLEWriteCharacteristic) {
	return []*server.BLEReadCharacteristic{
			newEndReadForwardChar(f),
			newReadRssiMapChar(f),
		}, []*server.BLEWriteCharacteristic{
			newWriteForwardChar(f),
			newStartReadForwardChar(f),
		}
}

// NewBLEForwarder is a function that creates a new ble forwarder
func NewBLEForwarder(name string, addr string, secret string, serverAddr string, serverListener models.BLEServerStatusListener, listener models.BLEForwarderListener) (*BLEForwarder, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	f := newBLEForwarder(addr, serverAddr, listener)
	readChars, writeChars := getChars(f)
	serv, err := server.NewBLEServerSharedDevice(d, name, secret, serverListener, readChars, writeChars)
	if err != nil {
		return nil, err
	}
	clien, err := client.NewBLEClientSharedDevice(d, addr, secret, serverAddr, func(attempts int, rssi int) {}, noop)
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
	mutex := &sync.Mutex{}
	for {
		time.Sleep(client.ScanInterval)
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

func wrapError(e1, e2 error) error {
	if e1 == nil {
		return e2
	}
	if e2 == nil {
		return e1
	}
	return errors.Wrap(e1, e2.Error())
}

func (forwarder *BLEForwarder) onScanned(a ble.Advertisement) error {
	rssi := a.RSSI()
	addr := a.Address().String()
	forwarder.rssiMap.Set(forwarder.addr, addr, rssi)
	isF := client.IsForwarder(a)
	var err error
	if addr != forwarder.serverAddr && isF {
		err = forwarder.updateRssiMap(addr)
		e := forwarder.reconnect()
		err = wrapError(err, e)
	}
	if addr == forwarder.serverAddr || isF {
		e := forwarder.refreshShortestPath()
		err = wrapError(err, e)
	}
	return err
}

func (forwarder *BLEForwarder) updateRssiMap(addr string) error {
	err := forwarder.keepTryConnect(addr)
	if err != nil {
		return err
	}
	data, err := forwarder.forwardingClient.ReadValue(ReadRssiMapCharUUID)
	if err != nil {
		return err
	}
	rssiMap, err := models.GetRssiMapFromBytes(data)
	if err != nil {
		return err
	}
	forwarder.rssiMap.Merge(rssiMap)
	return nil
}

func (forwarder *BLEForwarder) reconnect() error {
	if forwarder.toConnectAddr == "" {
		return nil
	}
	return forwarder.keepTryConnect(forwarder.toConnectAddr)
}

func (forwarder *BLEForwarder) refreshShortestPath() error {
	path, err := util.ShortestPath(*forwarder.rssiMap, forwarder.addr, forwarder.serverAddr)
	if err != nil {
		return err
	}
	if len(path) < 2 {
		return fmt.Errorf("Invalid path to server: %s", path)
	}
	nextHop := path[1]
	if forwarder.toConnectAddr != nextHop {
		forwarder.toConnectAddr = nextHop
		err = forwarder.keepTryConnect(nextHop)
	}
	return err
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
			forwarder.readCharUUIDMutex.Lock()
			forwarder.readCharUUID = r.CharUUID
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
		data, err := forwarder.forwardingClient.ReadValue(forwarder.readCharUUID)
		forwarder.readCharUUIDMutex.Unlock()
		return data, err
	}, noop}
}
