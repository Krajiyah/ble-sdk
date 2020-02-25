package client

import (
	"fmt"
	"github.com/pkg/errors"
	"strconv"
	"sync"
	"time"

	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/currantlabs/ble"
	"github.com/currantlabs/ble/linux"
	"golang.org/x/net/context"
)

const (
	// ScanInterval is the rate at which ble clients scan for ble server
	ScanInterval = time.Millisecond * 500
	// PingInterval is the rate at which ble clients will let ble server know of its state
	PingInterval = time.Second * 1
	// ForwardedReadDelay is the delay between start and end read requests
	ForwardedReadDelay   = time.Millisecond * 500
	afterConnectionDelay = time.Millisecond * 250
	maxRetryAttempts     = 5
)

type bleConnector interface {
	Connect(context.Context, ble.AdvFilter) (ble.Client, error)
	Scan(context.Context, bool, ble.AdvHandler, ble.AdvFilter) error
}

type stdBleConnector struct{}

// Connect implemented via BLE core lib
func (bc stdBleConnector) Connect(ctx context.Context, f ble.AdvFilter) (ble.Client, error) {
	return ble.Connect(ctx, f)
}

// Scan implemented via BLE core lib
func (bc stdBleConnector) Scan(ctx context.Context, b bool, h ble.AdvHandler, f ble.AdvFilter) error {
	return ble.Scan(ctx, b, h, f)
}

// BLEClientInt is a interface used to abstract BLEClient
type BLEClientInt interface {
	RawScanWithDuration(time.Duration, func(ble.Advertisement)) error
	RawScan(func(ble.Advertisement)) error
	ReadValue(string) ([]byte, error)
	RawConnect(ble.AdvFilter) error
	WriteValue(string, []byte) error
}

type BLEClientListener interface {
	OnConnected(string, int, int)
	OnDisconnected()
	OnTimeSync()
}

// BLEClient is a struct used to handle client connection to BLEServer
type BLEClient struct {
	name               string
	addr               string
	secret             string
	status             BLEClientStatus
	connectionAttempts int
	connectionMutex    *sync.Mutex
	timeSync           *util.TimeSync
	serverAddr         string
	connectedAddr      string
	rssiMap            *RssiMap
	ctx                context.Context
	cln                *ble.Client
	characteristics    map[string]*ble.Characteristic
	packetAggregator   util.PacketAggregator
	listener           BLEClientListener
	bleConnector       bleConnector
}

func newBLEClient(name string, addr string, secret string, serverAddr string, listener BLEClientListener) *BLEClient {
	rm := NewRssiMap()
	return &BLEClient{
		name, addr, secret, Disconnected, 0, &sync.Mutex{}, nil, serverAddr, "", rm, util.MakeINFContext(), nil,
		map[string]*ble.Characteristic{}, util.NewPacketAggregator(), listener,
		stdBleConnector{},
	}
}

// NewBLEClient is a function that creates a new ble client
func NewBLEClient(name string, addr string, secret string, serverAddr string, listener BLEClientListener) (*BLEClient, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	return NewBLEClientSharedDevice(d, name, addr, secret, serverAddr, listener)
}

// NewBLEClientSharedDevice is a function that creates a new ble client
func NewBLEClientSharedDevice(device ble.Device, name string, addr string, secret string, serverAddr string, listener BLEClientListener) (*BLEClient, error) {
	ble.SetDefaultDevice(device)
	return newBLEClient(name, addr, secret, serverAddr, listener), nil
}

// Run is a method that runs the connection from client to service
func (client *BLEClient) Run() {
	client.connectLoop()
	time.Sleep(afterConnectionDelay)
	go client.scan()
	go client.pingLoop()
}

// UnixTS returns the current time synced timestamp from the ble service
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

// Log writes a log object to the ble server's log characteristic
func (client *BLEClient) Log(log ClientLogRequest) error {
	b, _ := log.Data()
	return client.WriteValue(util.ClientLogUUID, b)
}

func (client *BLEClient) GetRssiMap() *RssiMap {
	return client.rssiMap
}

func (client *BLEClient) GetConnectionGraph() *ConnectionGraph {
	return NewConnectionGraphFromRaw(map[string]string{client.addr: client.connectedAddr})
}

func (client *BLEClient) isConnectedToForwarder() bool {
	return client.connectedAddr != "" && !util.AddrEqualAddr(client.serverAddr, client.connectedAddr)
}

// ReadValue will read packeted data from ble server from given uuid
func (client *BLEClient) ReadValue(uuid string) ([]byte, error) {
	if !client.isConnectedToForwarder() {
		return client.readValue(uuid)
	}
	req := ForwarderRequest{uuid, nil, true, false}
	data, err := req.Data()
	if err != nil {
		return nil, err
	}
	err = client.writeValue(util.StartReadForwardCharUUID, data)
	if err != nil {
		return nil, err
	}
	time.Sleep(ForwardedReadDelay)
	return client.readValue(util.EndReadForwardCharUUID)
}

func (client *BLEClient) readValue(uuid string) ([]byte, error) {
	c, err := client.getCharacteristic(uuid)
	if err != nil {
		return nil, err
	}
	guid := ""
	for !client.packetAggregator.HasDataFromPacketStream(guid) {
		packetData, err := client.optimizedReadChar(c)
		if err != nil {
			return nil, err
		}
		guid, err = client.packetAggregator.AddPacketFromPacketBytes(packetData)
		if err != nil {
			return nil, err
		}
	}
	encData, err := client.packetAggregator.PopAllDataFromPackets(guid)
	if err != nil {
		return nil, err
	}
	return util.Decrypt(encData, client.secret)
}

func isDataForwarderRequest(data []byte) bool {
	_, err := GetForwarderRequestFromBytes(data)
	return err == nil
}

func isForwardedWrite(uuid string, data []byte) bool {
	return isDataForwarderRequest(data) && util.AddrEqualAddr(uuid, util.WriteForwardCharUUID)
}

// WriteValue will write data (which is parsed to packets) to ble server to given uuid
func (client *BLEClient) WriteValue(uuid string, data []byte) error {
	if !client.isConnectedToForwarder() || isForwardedWrite(uuid, data) {
		return client.writeValue(uuid, data)
	}
	req := ForwarderRequest{uuid, data, false, true}
	payload, err := req.Data()
	if err != nil {
		return err
	}
	return client.writeValue(util.WriteForwardCharUUID, payload)
}

func (client *BLEClient) writeValue(uuid string, data []byte) error {
	if data == nil || len(data) == 0 {
		return errors.New("Empty data provided. Will skip writing.")
	}
	c, err := client.getCharacteristic(uuid)
	if err != nil {
		return err
	}
	encData, err := util.Encrypt(data, client.secret)
	if err != nil {
		return err
	}
	guid, err := client.packetAggregator.AddData(encData)
	if err != nil {
		return err
	}
	isLastPacket := false
	for !isLastPacket {
		var packetData []byte
		packetData, isLastPacket, err = client.packetAggregator.PopPacketDataFromStream(guid)
		if err != nil {
			return err
		}
		err = client.optimizedWriteChar(c, packetData)
		if err != nil {
			return err
		}
	}
	return nil
}

func (client *BLEClient) retryReadWrite(logic func() error) error {
	err := logic()
	if err != nil {
		err = retry(func() error {
			client.connectLoop()
			return logic()
		})
	}
	return err
}

func (client *BLEClient) optimizedReadChar(c *ble.Characteristic) ([]byte, error) {
	var data []byte
	err := client.retryReadWrite(func() error {
		return util.Optimize(func() error {
			dat, e := (*client.cln).ReadCharacteristic(c)
			data = dat
			return e
		})
	})
	return data, err
}

func (client *BLEClient) optimizedWriteChar(c *ble.Characteristic, data []byte) error {
	return client.retryReadWrite(func() error {
		return util.Optimize(func() error {
			return (*client.cln).WriteCharacteristic(c, data, true)
		})
	})
}

// IsForwarder is a filter which indicates if advertisement is from BLEForwarder
func IsForwarder(a ble.Advertisement) bool {
	for _, service := range a.Services() {
		if util.UuidEqualStr(service, util.MainServiceUUID) {
			return true
		}
	}
	return false
}

func (client *BLEClient) wrapFilter(fn func(ble.Advertisement) bool) func(ble.Advertisement) bool {
	return func(a ble.Advertisement) bool {
		addr := a.Address().String()
		rssi := a.RSSI()
		client.rssiMap.Set(client.addr, addr, rssi)
		b := fn(a)
		if b {
			client.connectedAddr = addr
		}
		return b
	}
}

func (client *BLEClient) serverFilter(a ble.Advertisement) bool {
	return util.AddrEqualAddr(a.Address().String(), client.serverAddr)
}

// RawScan exposes underlying BLE scanner
func (client *BLEClient) RawScan(handle func(ble.Advertisement)) error {
	return client.scanWithCtx(client.ctx, handle)
}

// RawScanWithDuration exposes underlying BLE scanner (with timeout)
func (client *BLEClient) RawScanWithDuration(duration time.Duration, handle func(ble.Advertisement)) error {
	ctx, _ := context.WithTimeout(context.Background(), duration)
	return client.scanWithCtx(ctx, handle)
}

func (client *BLEClient) scanWithCtx(ctx context.Context, handle func(ble.Advertisement)) error {
	return client.bleConnector.Scan(ctx, true, handle, nil)
}

func (client *BLEClient) scan() {
	for {
		time.Sleep(ScanInterval)
		client.RawScan(func(a ble.Advertisement) {
			rssi := a.RSSI()
			addr := a.Address().String()
			client.rssiMap.Set(client.addr, addr, rssi)
		})
	}
}

func (client *BLEClient) connectLoop() {
	client.connectionMutex.Lock()
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
	rssi, _ := client.rssiMap.Get(client.addr, client.connectedAddr)
	client.listener.OnConnected(client.connectedAddr, client.connectionAttempts, rssi)
	client.connectionMutex.Unlock()
}

func (client *BLEClient) pingLoop() {
	for {
		time.Sleep(PingInterval)
		if client.status != Connected {
			continue
		}
		m := client.rssiMap.GetAll()
		req := &ClientStateRequest{Name: client.name, Addr: client.addr, ConnectedAddr: client.connectedAddr, RssiMap: m}
		b, _ := req.Data()
		err := client.WriteValue(util.ClientStateUUID, b)
		if err != nil {
			continue
		}
		if client.timeSync == nil {
			client.syncTime()
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

func (client *BLEClient) rawConnect(filter ble.AdvFilter) error {
	if client.cln != nil {
		(*client.cln).CancelConnection()
	}
	cln, err := client.bleConnector.Connect(client.ctx, filter)
	client.cln = &cln
	if err != nil {
		return err
	}
	_, err = cln.ExchangeMTU(util.MTU)
	if err != nil {
		return err
	}
	p, err := cln.DiscoverProfile(true)
	if err != nil {
		return err
	}
	for _, s := range p.Services {
		if util.UuidEqualStr(s.UUID, util.MainServiceUUID) {
			for _, c := range s.Characteristics {
				uuid := util.UuidToStr(c.UUID)
				client.characteristics[uuid] = c
			}
			break
		}
	}
	return nil
}

// RawConnect exposes underlying ble connection functionality
func (client *BLEClient) RawConnect(filter ble.AdvFilter) error {
	var err error
	util.TryCatchBlock{
		Try: func() {
			err = client.rawConnect(filter)
		},
		Catch: func(e error) {
			err = e
		},
	}.Do()
	return err
}

func retry(fn func() error) error {
	err := errors.New("not error")
	attempts := 0
	for err != nil && attempts < maxRetryAttempts {
		if attempts > 0 {
			fmt.Printf("Error: %s\n Retrying...\n", err.Error())
		}
		err = fn()
		attempts += 1
	}
	if err != nil {
		fmt.Println("Exceeded connection attempts, and could not connect :(")
		return err
	}
	return nil
}

func (client *BLEClient) connect() error {
	serverFilter := client.wrapFilter(client.serverFilter)
	forwarderFilter := client.wrapFilter(IsForwarder)

	err := retry(func() error {
		return client.RawConnect(serverFilter)
	})

	if err != nil {
		fmt.Printf("Could not connect to server: %s\nSo now trying to connect to a forwarder.\n", err.Error())
		err = retry(func() error {
			return client.RawConnect(forwarderFilter)
		})
	}

	return err
}

func (client *BLEClient) getCharacteristic(uuid string) (*ble.Characteristic, error) {
	if c, ok := client.characteristics[uuid]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("No such uuid (%s) in characteristics (%v) advertised from server.", uuid, client.characteristics)
}
