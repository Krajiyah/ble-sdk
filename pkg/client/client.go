package client

import (
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"

	. "github.com/Krajiyah/ble-sdk/pkg/ble"
	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
	"golang.org/x/net/context"
)

const (
	ScanInterval         = time.Millisecond * 500
	PingInterval         = time.Second * 1
	ForwardedReadDelay   = time.Millisecond * 500
	afterConnectionDelay = time.Millisecond * 250
)

type Client interface {
	GetConnection() Connection
	WriteValue(string, []byte) error
	ReadValue(string) ([]byte, error)
	Log(ClientLogRequest) error
	UnixTS() (int64, error)
	Run()
}

type BLEClient struct {
	name                string
	addr                string
	secret              string
	status              BLEClientStatus
	connectionAttempts  int
	connectionLoopMutex *sync.Mutex
	timeSync            *util.TimeSync
	serverAddr          string
	ctx                 context.Context
	connection          Connection
	listener            BLEClientListener
}

func newBLEClient(name string, addr string, secret string, serverAddr string, listener BLEClientListener) *BLEClient {
	return &BLEClient{
		name, addr, secret, Disconnected, 0,
		&sync.Mutex{}, nil, serverAddr,
		util.MakeINFContext(), NewRealConnection(addr, secret), listener,
	}
}

func NewBLEClient(name string, addr string, secret string, serverAddr string, listener BLEClientListener) (*BLEClient, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	return NewBLEClientSharedDevice(d, name, addr, secret, serverAddr, listener)
}

func NewBLEClientSharedDevice(device ble.Device, name string, addr string, secret string, serverAddr string, listener BLEClientListener) (*BLEClient, error) {
	ble.SetDefaultDevice(device)
	return newBLEClient(name, addr, secret, serverAddr, listener), nil
}

func (client *BLEClient) Run() {
	client.connectLoop()
	time.Sleep(afterConnectionDelay)
	go client.scanLoop()
	go client.pingLoop()
}

func (client *BLEClient) scanLoop() {
	for {
		time.Sleep(ScanInterval)
		err := client.connection.Scan(client.ctx, func(a ble.Advertisement) {})
		if err != nil {
			client.listener.OnInternalError(err)
		}
	}
}

func (client *BLEClient) pingLoop() {
	for {
		time.Sleep(PingInterval)
		if client.status != Connected {
			continue
		}
		m := client.connection.GetRssiMap().GetAll()
		connectedAddr := client.connection.GetConnectedAddr()
		req := &ClientStateRequest{Name: client.name, Addr: client.addr, ConnectedAddr: connectedAddr, RssiMap: m}
		b, _ := req.Data()
		err := client.WriteValue(util.ClientStateUUID, b)
		if err != nil {
			client.listener.OnInternalError(err)
			continue
		}
		if client.timeSync == nil {
			err := client.syncTime()
			client.listener.OnInternalError(err)
		}
	}
}

func (client *BLEClient) syncTime() error {
	initTS, err := client.getUnixTS()
	if err != nil {
		return err
	}
	timeSync := util.NewTimeSync(initTS)
	client.timeSync = &timeSync
	client.listener.OnTimeSync()
	return nil
}

func (client *BLEClient) UnixTS() (int64, error) {
	if client.timeSync == nil {
		return 0, errors.New("Time not syncronized yet")
	}
	return client.timeSync.TS(), nil
}

func (client *BLEClient) getUnixTS() (int64, error) {
	b, err := client.ReadValue(util.TimeSyncUUID)
	if err != nil {
		return 0, err
	}
	ret, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return 0, err
	}
	return ret, nil
}

func (client *BLEClient) Log(log ClientLogRequest) error {
	b, _ := log.Data()
	return client.WriteValue(util.ClientLogUUID, b)
}

func (client *BLEClient) GetConnection() Connection { return client.connection }

func (client *BLEClient) isConnectedToForwarder() bool {
	connectedAddr := client.connection.GetConnectedAddr()
	return connectedAddr != "" && !util.AddrEqualAddr(client.serverAddr, connectedAddr)
}

func (client *BLEClient) ReadValue(uuid string) ([]byte, error) {
	if !client.isConnectedToForwarder() {
		return client.connection.ReadValue(uuid)
	}
	req := ForwarderRequest{uuid, nil, true, false}
	data, err := req.Data()
	if err != nil {
		return nil, err
	}
	err = client.connection.WriteValue(util.StartReadForwardCharUUID, data)
	if err != nil {
		return nil, err
	}
	time.Sleep(ForwardedReadDelay) // FIXME: this is too brittle
	return client.connection.ReadValue(util.EndReadForwardCharUUID)
}

func isDataForwarderRequest(data []byte) bool {
	_, err := GetForwarderRequestFromBytes(data)
	return err == nil
}

func isForwardedWrite(uuid string, data []byte) bool {
	return isDataForwarderRequest(data) && util.AddrEqualAddr(uuid, util.WriteForwardCharUUID)
}

func (client *BLEClient) WriteValue(uuid string, data []byte) error {
	if !client.isConnectedToForwarder() || isForwardedWrite(uuid, data) {
		return client.connection.WriteValue(uuid, data)
	}
	req := ForwarderRequest{uuid, data, false, true}
	payload, err := req.Data()
	if err != nil {
		return err
	}
	return client.connection.WriteValue(util.WriteForwardCharUUID, payload)
}

func HasMainService(a ble.Advertisement) bool {
	for _, service := range a.Services() {
		if util.UuidEqualStr(service, util.MainServiceUUID) {
			return true
		}
	}
	return false
}

func (client *BLEClient) connectLoop() {
	client.connectionLoopMutex.Lock()
	defer client.connectionLoopMutex.Unlock()
	client.status = Disconnected
	if client.connectionAttempts > 0 {
		client.listener.OnDisconnected()
	}
	client.connectionAttempts = 0
	err := errors.New("")
	for err != nil {
		client.connectionAttempts++
		err = client.connect()
	}
	client.status = Connected
	connectedAddr := client.connection.GetConnectedAddr()
	rssi, _ := client.connection.GetRssiMap().Get(client.addr, connectedAddr)
	client.listener.OnConnected(connectedAddr, client.connectionAttempts, rssi)
}

func (client *BLEClient) connect() error {
	err := client.connection.Connect(client.ctx, func(a ble.Advertisement) bool {
		return util.AddrEqualAddr(a.Addr().String(), client.serverAddr)
	})
	if err != nil {
		client.listener.OnInternalError(errors.Wrap(err, "Could not connect to server: %s\nSo now trying to connect to a forwarder.\n"))
		err = client.connection.Connect(client.ctx, HasMainService)
	}
	return err
}
