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
	scanDuration         = time.Second * 2
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
	connectionGraph   *models.ConnectionGraph
	readCharUUIDMutex *sync.Mutex
	readCharUUID      string
	listener          models.BLEForwarderListener
}

func newBLEForwarder(addr, serverAddr string, listener models.BLEForwarderListener) *BLEForwarder {
	return &BLEForwarder{
		addr, nil, nil,
		serverAddr, "", "",
		models.NewRssiMap(), models.NewConnectionGraph(),
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

type forwarderServerListener struct {
	listener models.BLEForwarderListener
}

func (l *forwarderServerListener) OnClientStateMapChanged(*models.ConnectionGraph, *models.RssiMap, map[string]models.BLEClientState) {
}
func (l *forwarderServerListener) OnClientLog(models.ClientLogRequest) {}
func (l *forwarderServerListener) OnReadOrWriteError(err error)        { l.listener.OnReadOrWriteError(err) }
func (l *forwarderServerListener) OnServerStatusChanged(s models.BLEServerStatus, err error) {
	l.listener.OnServerStatusChanged(s, err)
}

// NewBLEForwarder is a function that creates a new ble forwarder
func NewBLEForwarder(name string, addr string, secret string, serverAddr string, listener models.BLEForwarderListener) (*BLEForwarder, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	f := newBLEForwarder(addr, serverAddr, listener)
	readChars, writeChars := getChars(f)
	serv, err := server.NewBLEServerSharedDevice(d, name, addr, secret, &forwarderServerListener{listener: listener}, readChars, writeChars)
	if err != nil {
		return nil, err
	}
	clien, err := client.NewBLEClientSharedDevice(d, name, addr, secret, serverAddr, listener)
	if err != nil {
		return nil, err
	}
	f.forwardingServer = serv
	f.forwardingClient = clien
	return f, nil
}

// GetRssiMap returns underlying data for rssi map
func (forwarder *BLEForwarder) GetRssiMap() map[string]map[string]int {
	return forwarder.rssiMap.GetAll()
}

// GetConnectionGraph returns underlying data for connection graph
func (forwarder *BLEForwarder) GetConnectionGraph() map[string]string {
	return forwarder.connectionGraph.GetAll()
}

// GetClient returns the client abstraction for the forwarder
func (forwarder *BLEForwarder) GetClient() *client.BLEClient {
	return forwarder.forwardingClient.(*client.BLEClient)
}

// GetServer returns the server abstraction for the forwarder
func (forwarder *BLEForwarder) GetServer() *server.BLEServer {
	return forwarder.forwardingServer.(*server.BLEServer)
}

// Run is a method that runs the forwarder forever
func (forwarder *BLEForwarder) Run() error {
	go forwarder.scanLoop()
	return forwarder.forwardingServer.Run()
}

func (forwarder *BLEForwarder) collectAdvirtisements() ([]ble.Advertisement, error) {
	ret := []ble.Advertisement{}
	mutex := sync.Mutex{}
	err := forwarder.forwardingClient.RawScanWithDuration(scanDuration, func(a ble.Advertisement) {
		mutex.Lock()
		ret = append(ret, a)
		mutex.Unlock()
	})
	if err != nil && err.Error() == "context deadline exceeded" {
		err = nil
	}
	return ret, err
}

func (forwarder *BLEForwarder) scanLoop() {
	for {
		time.Sleep(client.ScanInterval)
		advs, err := forwarder.collectAdvirtisements()
		if err != nil {
			e := errors.Wrap(err, "collectAdvirtisements error")
			forwarder.listener.OnConnectionError(e)
		}
		for _, a := range advs {
			err := forwarder.onScanned(a)
			if err != nil {
				e := errors.Wrap(err, "onScanned error")
				forwarder.listener.OnError(e)
			}
		}
	}
}

func (forwarder *BLEForwarder) onScanned(a ble.Advertisement) error {
	rssi := a.RSSI()
	addr := a.Address().String()
	forwarder.rssiMap.Set(forwarder.addr, addr, rssi)
	if !client.IsForwarder(a) {
		return nil
	}
	err := forwarder.keepTryConnect(addr)
	if err != nil {
		return err
	}
	if util.AddrEqualAddr(addr, forwarder.serverAddr) {
		fmt.Println("Updating client state")
		err = forwarder.updateClientState()
	} else {
		err = forwarder.updateNetworkState(addr)
	}
	if err != nil {
		return err
	}
	if forwarder.toConnectAddr != "" && !util.AddrEqualAddr(forwarder.toConnectAddr, addr) {
		err = forwarder.keepTryConnect(forwarder.toConnectAddr)
	}
	if err != nil {
		return err
	}
	return forwarder.refreshShortestPath()
}

func (forwarder *BLEForwarder) updateClientState() error {
	r := models.ClientStateRequest{Addr: forwarder.addr, ConnectedAddr: forwarder.serverAddr, RssiMap: forwarder.rssiMap.GetAll()}
	data, err := r.Data()
	if err != nil {
		return err
	}
	fmt.Println("Writing to " + forwarder.connectedAddr)
	return forwarder.forwardingClient.WriteValue(util.ClientStateUUID, data)
}

func (forwarder *BLEForwarder) updateNetworkState(addr string) error {
	data, err := forwarder.forwardingClient.ReadValue(util.ReadRssiMapCharUUID)
	if err != nil {
		return err
	}
	rssiMap, err := models.GetRssiMapFromBytes(data)
	if err != nil {
		return err
	}
	forwarder.rssiMap.Merge(rssiMap)
	data, err = forwarder.forwardingClient.ReadValue(util.ReadConnectionGraphUUID)
	if err != nil {
		return err
	}
	connectionGraph, err := models.GetConnectionGraphFromBytes(data)
	if err != nil {
		return err
	}
	forwarder.connectionGraph.Merge(connectionGraph)
	return nil
}

func (forwarder *BLEForwarder) refreshShortestPath() error {
	path, err := util.ShortestPath(forwarder.rssiMap.GetAll(), forwarder.addr, forwarder.serverAddr)
	if err != nil {
		return errors.Wrap(err, "Could not calc shortest path.")
	}
	if len(path) < 2 {
		return fmt.Errorf("Invalid path to server: %s", path)
	}
	nextHop := path[1]
	if !util.AddrEqualAddr(forwarder.toConnectAddr, nextHop) {
		forwarder.toConnectAddr = nextHop
		err = forwarder.keepTryConnect(nextHop)
	}
	return err
}

func (forwarder *BLEForwarder) keepTryConnect(addr string) error {
	fmt.Println("Connecting to " + addr)
	err := errors.New("")
	attempts := 0
	rssi := 0
	for err != nil && attempts < maxConnectAttempts {
		rssi, err = forwarder.connect(addr)
		if err != nil {
			e := errors.Wrap(err, "keepTryConnect single connection error")
			forwarder.listener.OnConnectionError(e)
		}
		attempts++
	}
	forwarder.listener.OnConnected(addr, attempts, rssi)
	return nil
}

func (forwarder *BLEForwarder) connect(addr string) (int, error) {
	forwarder.connectedAddr = ""
	rssi := 0
	err := forwarder.forwardingClient.RawConnect(func(a ble.Advertisement) bool {
		b := util.AddrEqualAddr(a.Address().String(), addr)
		if b {
			rssi = a.RSSI()
		}
		return b
	})
	if err != nil {
		return 0, err
	}
	forwarder.connectedAddr = addr
	forwarder.connectionGraph.Set(forwarder.addr, addr)
	return rssi, nil
}

func (forwarder *BLEForwarder) isConnected() bool {
	return forwarder.connectedAddr != ""
}

func (forwarder *BLEForwarder) isConnectedToServer() bool {
	return util.AddrEqualAddr(forwarder.connectedAddr, forwarder.serverAddr)
}

func noop() {}

func newReadRssiMapChar(forwarder *BLEForwarder) *server.BLEReadCharacteristic {
	return &server.BLEReadCharacteristic{Uuid: util.ReadRssiMapCharUUID, HandleRead: func(addr string, ctx context.Context) ([]byte, error) {
		return forwarder.rssiMap.Data()
	}, DoInBackground: noop}
}

func newReadConnectionGraphChar(forwarder *BLEForwarder) *server.BLEReadCharacteristic {
	return &server.BLEReadCharacteristic{Uuid: util.ReadConnectionGraphUUID, HandleRead: func(addr string, ctx context.Context) ([]byte, error) {
		return forwarder.connectionGraph.Data()
	}, DoInBackground: noop}
}

func newWriteForwardChar(forwarder *BLEForwarder) *server.BLEWriteCharacteristic {
	return &server.BLEWriteCharacteristic{Uuid: util.WriteForwardCharUUID, HandleWrite: func(addr string, data []byte, err error) {
		if err != nil {
			forwarder.listener.OnReadOrWriteError(err)
			return
		}
		if !forwarder.isConnected() {
			forwarder.listener.OnConnectionError(errors.New(errNotConnected))
			return
		}
		if !forwarder.isConnectedToServer() {
			err := forwarder.forwardingClient.WriteValue(util.WriteForwardCharUUID, data)
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
	}, DoInBackground: noop}
}

func newStartReadForwardChar(forwarder *BLEForwarder) *server.BLEWriteCharacteristic {
	return &server.BLEWriteCharacteristic{Uuid: util.StartReadForwardCharUUID, HandleWrite: func(addr string, data []byte, err error) {
		if err != nil {
			forwarder.listener.OnReadOrWriteError(err)
			return
		}
		if !forwarder.isConnected() {
			forwarder.listener.OnConnectionError(errors.New(errNotConnected))
			return
		}
		if !forwarder.isConnectedToServer() {
			err := forwarder.forwardingClient.WriteValue(util.StartReadForwardCharUUID, data)
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
	}, DoInBackground: noop}
}

func newEndReadForwardChar(forwarder *BLEForwarder) *server.BLEReadCharacteristic {
	return &server.BLEReadCharacteristic{Uuid: util.EndReadForwardCharUUID, HandleRead: func(addr string, ctx context.Context) ([]byte, error) {
		if !forwarder.isConnected() {
			return nil, errors.New(errNotConnected)
		}
		if !forwarder.isConnectedToServer() {
			return forwarder.forwardingClient.ReadValue(util.EndReadForwardCharUUID)
		}
		data, err := forwarder.forwardingClient.ReadValue(forwarder.readCharUUID)
		forwarder.readCharUUIDMutex.Unlock()
		return data, err
	}, DoInBackground: noop}
}
