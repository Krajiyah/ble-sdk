package client

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"

	. "github.com/Krajiyah/ble-sdk/pkg/ble"
	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
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
	connectionLoopMutex *sync.Mutex
	timeSync            *util.TimeSync
	serverAddr          string
	connection          Connection
	listener            BLEClientListener
}

func NewBLEClientWithSharedConn(name string, addr string, secret string, serverAddr string, listener BLEClientListener, conn Connection) (*BLEClient, error) {
	return &BLEClient{
		name, addr, secret, Disconnected,
		&sync.Mutex{}, nil, serverAddr,
		conn, listener,
	}, nil
}

func NewBLEClient(name string, addr string, secret string, serverAddr string, timeout time.Duration, listener BLEClientListener) (*BLEClient, error) {
	conn, err := NewRealConnection(addr, secret, timeout, listener, nil)
	if err != nil {
		return nil, err
	}
	return NewBLEClientWithSharedConn(name, addr, secret, serverAddr, listener, conn)
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
		err := client.connection.Scan(func(_ ble.Advertisement) {})
		if err != nil {
			client.listener.OnInternalError(err)
		}
	}
}

func (client *BLEClient) ping(mutex *sync.Mutex) error {
	mutex.Lock()
	defer mutex.Unlock()
	if client.status != Connected {
		return nil
	}
	m := client.connection.GetRssiMap().GetAll()
	connectedAddr := client.connection.GetConnectedAddr()
	req := &ClientStateRequest{Name: client.name, Addr: client.addr, ConnectedAddr: connectedAddr, RssiMap: m}
	b, _ := req.Data()
	err := client.WriteValue(util.ClientStateUUID, b)
	if err != nil {
		return err
	}
	if client.timeSync == nil {
		return client.syncTime()
	}
	return nil
}

func (client *BLEClient) pingLoop() {
	mutex := &sync.Mutex{}
	for {
		time.Sleep(PingInterval)
		if err := client.ping(mutex); err != nil {
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
	err := errors.New("")
	for err != nil {
		err = client.connect()
		if err != nil {
			fmt.Println("ConnectLoop connect issue: ")
			fmt.Println(err)
		}
	}
	client.status = Connected
}

func (client *BLEClient) connect() error {
	err := client.connection.Dial(client.serverAddr)
	if err != nil {
		client.listener.OnInternalError(errors.Wrap(err, "Could not connect to server.\nSo now trying to connect to a forwarder.\n"))
		err = client.connection.Connect(HasMainService)
	}
	return err
}
