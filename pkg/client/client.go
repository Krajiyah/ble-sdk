package client

import (
	"errors"
	"strconv"
	"strings"
	"time"

	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/server"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/currantlabs/ble"
	"golang.org/x/net/context"
)

const (
	// ScanInterval is the rate at which ble clients scan for ble server
	ScanInterval = time.Millisecond * 500
	// PingInterval is the rate at which ble clients will let ble server know of its state
	PingInterval = time.Second * 1
	inf          = 1000000
)

// BLEClient is a struct used to handle client connection to BLEServer
type BLEClient struct {
	addr               string
	secret             string
	status             BLEClientStatus
	connectionAttempts int
	timeSync           *util.TimeSync
	serverAddr         string
	rssiMap            map[string]int
	ctx                context.Context
	cln                *ble.Client
	characteristics    map[string]*ble.Characteristic
	packetAggregator   util.PacketAggregator
	onConnected        func(int, int)
	onDisconnected     func()
}

// NewBLEClient is a function that creates a new ble client
func NewBLEClient(addr string, secret string, serverAddr string, onConnected func(int, int), onDisconnected func()) (*BLEClient, error) {
	d, err := util.NewDevice()
	if err != nil {
		return nil, err
	}
	ble.SetDefaultDevice(d)
	return &BLEClient{
		addr, secret, Disconnected, 0, nil, serverAddr, map[string]int{}, makeINFContext(), nil,
		map[string]*ble.Characteristic{}, util.NewPacketAggregator(), onConnected, onDisconnected,
	}, nil
}

// Run is a method that runs the connection from client to service (forever)
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
func (client *BLEClient) ReadValue(uuid string) ([]byte, error) {
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
func (client *BLEClient) WriteValue(uuid string, data []byte) error {
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
	b := addrEqualAddr(a.Address().String(), client.serverAddr)
	if b {
		client.rssiMap[client.addr] = a.RSSI()
	}
	return b
}

func (client *BLEClient) scan() {
	client.rssiMap = map[string]int{}
	for {
		time.Sleep(ScanInterval)
		ble.Scan(client.ctx, true, func(a ble.Advertisement) {
			rssi := a.RSSI()
			addr := a.Address().String()
			client.rssiMap[addr] = rssi
			if addr == client.serverAddr {
				client.rssiMap[client.addr] = a.RSSI()
			}
		}, nil)
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
	client.onConnected(client.connectionAttempts, client.rssiMap[client.addr])
}

func (client *BLEClient) pingLoop() {
	for {
		time.Sleep(PingInterval)
		req := &ClientStateRequest{client.rssiMap}
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

func (client *BLEClient) connect() error {
	if client.cln != nil {
		(*client.cln).CancelConnection()
	}
	cln, err := ble.Connect(client.ctx, client.filter)
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
		if uuidEqualStr(s.UUID, server.MainServiceUUID) {
			for _, c := range s.Characteristics {
				client.characteristics[c.UUID.String()] = c
			}
			break
		}
	}
	return nil
}

func (client *BLEClient) getCharacteristic(uuid string) (*ble.Characteristic, error) {
	if c, ok := client.characteristics[uuid]; ok {
		return c, nil
	}
	return nil, errors.New("No such uuid in characteristics advertised from server")
}

func addrEqualAddr(a string, b string) bool {
	return strings.ToUpper(a) == strings.ToUpper(b)
}

func uuidEqualStr(u ble.UUID, s string) bool {
	compare := strings.Replace(s, "-", "", -1)
	return addrEqualAddr(compare, u.String())
}

func makeINFContext() context.Context {
	return ble.WithSigHandler(context.WithTimeout(context.Background(), inf*time.Hour))
}
