package client

import (
	"fmt"
	"strconv"
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
	lookForServerTime    = time.Second * 20
)

type Client interface {
	GetConnection() Connection
	WriteValue(string, []byte, bool) error
	ReadValue(string) ([]byte, error)
	Log(ClientLogRequest) error
	UnixTS() (int64, error)
	Run()
}

type BLEClient struct {
	name       string
	addr       string
	secret     string
	status     BLEClientStatus
	timeSync   *util.TimeSync
	serverAddr string
	connection Connection
	listener   BLEClientListener
}

func NewBLEClientWithSharedConn(name string, addr string, secret string, serverAddr string, listener BLEClientListener, conn Connection) (*BLEClient, error) {
	return &BLEClient{
		name, addr, secret, Disconnected,
		nil, serverAddr,
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
	client.connect()
	client.status = Connected
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

func (client *BLEClient) ping() error {
	if client.status != Connected {
		return nil
	}
	m := client.connection.GetRssiMap().GetAll()
	connectedAddr := client.connection.GetConnectedAddr()
	req := &ClientStateRequest{Name: client.name, Addr: client.addr, ConnectedAddr: connectedAddr, RssiMap: m}
	b, _ := req.Data()
	err := client.WriteValue(util.ClientStateUUID, b, false)
	if err != nil {
		return err
	}
	fmt.Println("Updated Client State!")
	if client.timeSync == nil {
		return client.syncTime()
	}
	return nil
}

func (client *BLEClient) pingLoop() {
	for {
		time.Sleep(PingInterval)
		if err := client.ping(); err != nil {
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
	return client.WriteValue(util.ClientLogUUID, b, false)
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
	client.connection.BlockingWriteValue(util.StartReadForwardCharUUID, data)
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

func (client *BLEClient) doWriteValue(uuid string, data []byte, block bool) error {
	if block {
		return client.connection.BlockingWriteValue(uuid, data)
	}
	client.connection.NonBlockingWriteValue(uuid, data)
	return nil
}

func (client *BLEClient) WriteValue(uuid string, data []byte, block bool) error {
	if !client.isConnectedToForwarder() || isForwardedWrite(uuid, data) {
		return client.doWriteValue(uuid, data, block)
	}
	req := ForwarderRequest{uuid, data, false, true}
	payload, err := req.Data()
	if err != nil {
		return err
	}
	return client.doWriteValue(util.WriteForwardCharUUID, payload, block)
}

func HasMainService(a ble.Advertisement) bool {
	for _, service := range a.Services() {
		if util.UuidEqualStr(service, util.MainServiceUUID) {
			return true
		}
	}
	return false
}

func (client *BLEClient) connect() {
	foundServer := false
	client.connection.ScanForDuration(lookForServerTime, func(a ble.Advertisement) {
		if util.AddrEqualAddr(a.Addr().String(), client.serverAddr) {
			foundServer = true
		}
	})
	if foundServer {
		fmt.Println("Found server! So connecting to it...")
		client.connection.Dial(client.serverAddr)
	} else {
		fmt.Println("Could not find server so looking for forwarder...")
		client.connection.Connect(HasMainService)
	}
}
