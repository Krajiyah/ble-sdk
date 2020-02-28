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
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
	"golang.org/x/net/context"
)

const (
	scanDuration         = time.Second * 2
	errNotConnected      = "Forwarder is not connected"
	errInvalidForwardReq = "Invalid forwarding request"
)

type BLEForwarder struct {
	name              string
	addr              string
	forwardingServer  server.Server
	forwardingClient  client.Client
	serverAddr        string
	toConnectAddr     string
	rssiMap           *models.RssiMap
	connectionGraph   *models.ConnectionGraph
	readCharUUIDMutex *sync.Mutex
	readCharUUID      string
	listener          models.BLEForwarderListener
	ctx               context.Context
}

func newBLEForwarder(name, addr, serverAddr string, listener models.BLEForwarderListener) *BLEForwarder {
	return &BLEForwarder{
		name, addr, nil, nil,
		serverAddr, "", models.NewRssiMap(), models.NewConnectionGraph(),
		&sync.Mutex{}, "",
		listener, context.Background(),
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
func (l *forwarderServerListener) OnInternalError(err error)           { l.listener.OnInternalError(err) }
func (l *forwarderServerListener) OnServerStatusChanged(s models.BLEServerStatus, err error) {
	l.listener.OnServerStatusChanged(s, err)
}

func NewBLEForwarder(name string, addr string, secret string, serverAddr string, listener models.BLEForwarderListener) (*BLEForwarder, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	f := newBLEForwarder(name, addr, serverAddr, listener)
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

func (forwarder *BLEForwarder) GetRssiMap() map[string]map[string]int {
	a := forwarder.forwardingServer.GetRssiMap()
	b := forwarder.forwardingClient.GetConnection().GetRssiMap()
	c := forwarder.rssiMap
	rm := models.NewRssiMapFromRaw(a.GetAll())
	rm.Merge(b)
	rm.Merge(c)
	return rm.GetAll()
}

func (forwarder *BLEForwarder) GetConnectionGraph() map[string]string {
	a := forwarder.forwardingServer.GetConnectionGraph()
	b := models.NewConnectionGraphFromRaw(map[string]string{forwarder.addr: forwarder.forwardingClient.GetConnection().GetConnectedAddr()})
	c := forwarder.connectionGraph
	cg := models.NewConnectionGraphFromRaw(a.GetAll())
	cg.Merge(b)
	cg.Merge(c)
	return cg.GetAll()
}

func (forwarder *BLEForwarder) GetClient() client.Client {
	return forwarder.forwardingClient
}

func (forwarder *BLEForwarder) Run() error {
	go forwarder.scanLoop()
	return forwarder.forwardingServer.Run()
}

func (forwarder *BLEForwarder) collectAdvirtisements() ([]ble.Advertisement, error) {
	ret := []ble.Advertisement{}
	mutex := sync.Mutex{}
	err := forwarder.forwardingClient.GetConnection().ScanForDuration(forwarder.ctx, scanDuration, func(a ble.Advertisement) {
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
			forwarder.listener.OnInternalError(e)
			continue
		}
		for _, a := range advs {
			err := forwarder.onScanned(a)
			if err != nil {
				e := errors.Wrap(err, "onScanned error")
				forwarder.listener.OnInternalError(e)
			}
		}
	}
}

func (forwarder *BLEForwarder) onScanned(a ble.Advertisement) error {
	addr := a.Addr().String()
	if !client.HasMainService(a) {
		return nil
	}
	err := forwarder.connect(addr)
	if err != nil {
		return err
	}
	if util.AddrEqualAddr(addr, forwarder.serverAddr) {
		err = forwarder.updateClientState()
	} else {
		err = forwarder.updateNetworkState(addr)
	}
	if err != nil {
		return err
	}
	err = forwarder.connect(forwarder.toConnectAddr)
	if err != nil {
		return err
	}
	return forwarder.refreshShortestPath()
}

func (forwarder *BLEForwarder) updateClientState() error {
	r := models.ClientStateRequest{
		Name:          forwarder.name,
		Addr:          forwarder.addr,
		ConnectedAddr: forwarder.serverAddr,
		RssiMap:       forwarder.GetRssiMap(),
	}
	data, err := r.Data()
	if err != nil {
		return err
	}
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
	path, err := util.ShortestPath(forwarder.GetRssiMap(), forwarder.addr, forwarder.serverAddr)
	if err != nil {
		return errors.Wrap(err, "Could not calc shortest path.")
	}
	if len(path) < 2 {
		return fmt.Errorf("Invalid path to server: %s", path)
	}
	nextHop := path[1]
	forwarder.toConnectAddr = nextHop
	return forwarder.connect(nextHop)
}

func (forwarder *BLEForwarder) connect(addr string) error {
	conn := forwarder.forwardingClient.GetConnection()
	if addr == "" || util.AddrEqualAddr(addr, conn.GetConnectedAddr()) {
		return nil
	}
	return conn.Dial(forwarder.ctx, addr)
}

func (forwarder *BLEForwarder) isConnected() bool {
	return forwarder.forwardingClient.GetConnection().GetConnectedAddr() != ""
}

func (forwarder *BLEForwarder) isConnectedToServer() bool {
	return util.AddrEqualAddr(forwarder.forwardingClient.GetConnection().GetConnectedAddr(), forwarder.serverAddr)
}

func noop() {}

func newReadRssiMapChar(forwarder *BLEForwarder) *server.BLEReadCharacteristic {
	return &server.BLEReadCharacteristic{Uuid: util.ReadRssiMapCharUUID, HandleRead: func(addr string, ctx context.Context) ([]byte, error) {
		return models.NewRssiMapFromRaw(forwarder.GetRssiMap()).Data()
	}, DoInBackground: noop}
}

func newReadConnectionGraphChar(forwarder *BLEForwarder) *server.BLEReadCharacteristic {
	return &server.BLEReadCharacteristic{Uuid: util.ReadConnectionGraphUUID, HandleRead: func(addr string, ctx context.Context) ([]byte, error) {
		return models.NewConnectionGraphFromRaw(forwarder.GetConnectionGraph()).Data()
	}, DoInBackground: noop}
}

func newWriteForwardChar(forwarder *BLEForwarder) *server.BLEWriteCharacteristic {
	return &server.BLEWriteCharacteristic{Uuid: util.WriteForwardCharUUID, HandleWrite: func(addr string, data []byte, err error) {
		if err != nil {
			forwarder.listener.OnInternalError(err)
			return
		}
		if !forwarder.isConnected() {
			forwarder.listener.OnInternalError(errors.New(errNotConnected))
			return
		}
		if !forwarder.isConnectedToServer() {
			err := forwarder.forwardingClient.WriteValue(util.WriteForwardCharUUID, data)
			if err != nil {
				forwarder.listener.OnInternalError(err)
				return
			}
		} else {
			r, err := models.GetForwarderRequestFromBytes(data)
			if err != nil {
				forwarder.listener.OnInternalError(err)
				return
			}
			if r.IsRead || !r.IsWrite {
				forwarder.listener.OnInternalError(errors.New(errInvalidForwardReq))
				return
			}
			err = forwarder.forwardingClient.WriteValue(r.CharUUID, r.Payload)
			if err != nil {
				forwarder.listener.OnInternalError(err)
				return
			}
		}
	}, DoInBackground: noop}
}

func newStartReadForwardChar(forwarder *BLEForwarder) *server.BLEWriteCharacteristic {
	return &server.BLEWriteCharacteristic{Uuid: util.StartReadForwardCharUUID, HandleWrite: func(addr string, data []byte, err error) {
		if err != nil {
			forwarder.listener.OnInternalError(err)
			return
		}
		if !forwarder.isConnected() {
			forwarder.listener.OnInternalError(errors.New(errNotConnected))
			return
		}
		if !forwarder.isConnectedToServer() {
			err := forwarder.forwardingClient.WriteValue(util.StartReadForwardCharUUID, data)
			if err != nil {
				forwarder.listener.OnInternalError(err)
				return
			}
		} else {
			r, err := models.GetForwarderRequestFromBytes(data)
			if err != nil {
				forwarder.listener.OnInternalError(err)
				return
			}
			if r.IsWrite || !r.IsRead {
				forwarder.listener.OnInternalError(errors.New(errInvalidForwardReq))
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
		if forwarder.readCharUUID != "" {
			data, err := forwarder.forwardingClient.ReadValue(forwarder.readCharUUID)
			forwarder.readCharUUID = ""
			forwarder.readCharUUIDMutex.Unlock()
			return data, err
		}
		return nil, errors.New("StartReadChar not invoked, so can not EndReadChar")
	}, DoInBackground: noop}
}
