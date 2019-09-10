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
	"github.com/currantlabs/ble/linux"
	"golang.org/x/net/context"
)

const (
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
	serverAddr         string
	rssiMap            map[string]int
	ctx                context.Context
	cln                ble.Client
	characteristics    map[string]*ble.Characteristic
	packetAggregator   util.PacketAggregator
	onConnected        func(int, int)
	onDisconnected     func()
}

// NewBLEClient is a function that creates a new ble client
func NewBLEClient(addr string, secret string, serverAddr string, onConnected func(int, int), onDisconnected func()) (*BLEClient, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	ble.SetDefaultDevice(d)
	client := BLEClient{}
	client.secret = secret
	client.packetAggregator = util.NewPacketAggregator()
	client.addr = addr
	client.status = Disconnected
	client.connectionAttempts = 0
	client.serverAddr = serverAddr
	client.rssiMap = map[string]int{}
	client.ctx = makeINFContext()
	client.characteristics = map[string]*ble.Characteristic{}
	client.onConnected = client.onConnected
	client.onDisconnected = client.onDisconnected
	return &client, nil
}

// Run is a method that runs the connection from client to service (forever)
func (client *BLEClient) Run() {
	client.connectLoop()
	go client.scan()
	go client.pingLoop()
}

// UnixTS returns the current time synced timestamp from the ble service
func (client *BLEClient) UnixTS() (int64, error) {
	b, err := client.readValue(client.characteristics[server.TimeSyncUUID])
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
	return client.writeValue(client.characteristics[server.ClientLogUUID], b)
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
		err := client.writeValue(client.characteristics[server.ClientStateUUID], b)
		if err != nil {
			client.connectLoop()
		}
	}
}

func (client *BLEClient) connect() error {
	if client.cln != nil {
		client.cln.CancelConnection()
	}
	cln, err := ble.Connect(client.ctx, client.filter)
	client.cln = cln
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

func (client *BLEClient) readValue(c *ble.Characteristic) ([]byte, error) {
	guid := ""
	for !client.packetAggregator.HasDataFromPacketStream(guid) {
		packetData, err := client.cln.ReadCharacteristic(c)
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

func (client *BLEClient) writeValue(c *ble.Characteristic, data []byte) error {
	guid, err := client.packetAggregator.AddData(data)
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
		err = client.cln.WriteCharacteristic(c, packetData, true)
		if err != nil {
			return err
		}
	}
	return nil
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
