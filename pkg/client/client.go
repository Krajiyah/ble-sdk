package client

import (
	"errors"
	"strconv"
	"time"

	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/server"
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
)

// BLEClientInt is a interface used to abstract BLEClient
type BLEClientInt interface {
	RawScan(func(ble.Advertisement)) error
	ReadValue(string) ([]byte, error)
	RawConnect(ble.AdvFilter) error
	WriteValue(string, []byte) error
}

// BLEClient is a struct used to handle client connection to BLEServer
type BLEClient struct {
	addr               string
	secret             string
	status             BLEClientStatus
	connectionAttempts int
	timeSync           *util.TimeSync
	serverAddr         string
	rssiMap            *RssiMap
	ctx                context.Context
	cln                *ble.Client
	characteristics    map[string]*ble.Characteristic
	packetAggregator   util.PacketAggregator
	onConnected        func(int, int)
	onDisconnected     func()
}

func newBLEClient(addr string, secret string, serverAddr string, onConnected func(int, int), onDisconnected func()) *BLEClient {
	return &BLEClient{
		addr, secret, Disconnected, 0, nil, serverAddr, &RssiMap{}, util.MakeINFContext(), nil,
		map[string]*ble.Characteristic{}, util.NewPacketAggregator(), onConnected, onDisconnected,
	}
}

// NewBLEClient is a function that creates a new ble client
func NewBLEClient(addr string, secret string, serverAddr string, onConnected func(int, int), onDisconnected func()) (*BLEClient, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	return NewBLEClientSharedDevice(d, addr, secret, serverAddr, onConnected, onDisconnected)
}

// NewBLEClientSharedDevice is a function that creates a new ble client
func NewBLEClientSharedDevice(device ble.Device, addr string, secret string, serverAddr string, onConnected func(int, int), onDisconnected func()) (*BLEClient, error) {
	ble.SetDefaultDevice(device)
	return newBLEClient(addr, secret, serverAddr, onConnected, onDisconnected), nil
}

// Run is a method that runs the connection from client to service
func (client *BLEClient) Run() {
	client.connectLoop()
	go client.scan()
	go client.pingLoop()
}

// UnixTS returns the current time synced timestamp from the ble service
func (client *BLEClient) UnixTS() int64 {
	return client.timeSync.TS()
}

func (client *BLEClient) getUnixTS() (int64, error) {
	b, err := client.ReadValue(server.TimeSyncUUID)
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
	return client.WriteValue(server.ClientLogUUID, b)
}

// ReadValue will read packeted data from ble server from given uuid
func (client BLEClient) ReadValue(uuid string) ([]byte, error) {
	c, err := client.getCharacteristic(uuid)
	if err != nil {
		return nil, err
	}
	guid := ""
	for !client.packetAggregator.HasDataFromPacketStream(guid) {
		packetData, err := (*client.cln).ReadCharacteristic(c)
		if err != nil {
			return nil, err
		}
		guid, err = client.packetAggregator.AddPacketFromPacketBytes(packetData)
	}
	encData, err := client.packetAggregator.PopAllDataFromPackets(guid)
	if err != nil {
		return nil, err
	}
	return util.Decrypt(encData, client.secret)
}

// WriteValue will write data (which is parsed to packets) to ble server to given uuid
func (client BLEClient) WriteValue(uuid string, data []byte) error {
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
		err = (*client.cln).WriteCharacteristic(c, packetData, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func (client *BLEClient) filter(a ble.Advertisement) bool {
	addr := a.Address().String()
	rssi := a.RSSI()
	client.rssiMap.Set(client.addr, addr, rssi)
	return util.AddrEqualAddr(addr, client.serverAddr)
}

// RawScan exposes underlying BLE scanner
func (client BLEClient) RawScan(handle func(ble.Advertisement)) error {
	return ble.Scan(client.ctx, true, handle, nil)
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
	client.status = Connected
	if client.connectionAttempts > 0 {
		client.status = Disconnected
		client.onDisconnected()
	}
	err := errors.New("")
	for err != nil {
		client.connectionAttempts++
		err = client.connect()
	}
	client.status = Connected
	client.onConnected(client.connectionAttempts, (*client.rssiMap)[client.addr][client.serverAddr])
}

func (client *BLEClient) pingLoop() {
	for {
		time.Sleep(PingInterval)
		req := &ClientStateRequest{*client.rssiMap}
		b, _ := req.Data()
		err := client.WriteValue(server.ClientStateUUID, b)
		if err != nil {
			client.connectLoop()
			continue
		}
		initTS, err := client.getUnixTS()
		if err != nil {
			client.connectLoop()
			continue
		}
		timeSync := util.NewTimeSync(initTS)
		client.timeSync = &timeSync
	}
}

func (client *BLEClient) rawConnect(filter ble.AdvFilter) error {
	if client.cln != nil {
		(*client.cln).CancelConnection()
	}
	cln, err := ble.Connect(client.ctx, filter)
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
		if util.UuidEqualStr(s.UUID, server.MainServiceUUID) {
			for _, c := range s.Characteristics {
				client.characteristics[c.UUID.String()] = c
			}
			break
		}
	}
	return nil
}

// RawConnect exposes underlying ble connection functionality
func (client BLEClient) RawConnect(filter ble.AdvFilter) error {
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

func (client *BLEClient) connect() error {
	return client.RawConnect(client.filter)
}

func (client *BLEClient) getCharacteristic(uuid string) (*ble.Characteristic, error) {
	if c, ok := client.characteristics[uuid]; ok {
		return c, nil
	}
	return nil, errors.New("No such uuid in characteristics advertised from server")
}
