package internal

import (
	"bytes"
	"errors"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
)

type TestListener struct{}

func (l *TestListener) OnDisconnected()       {}
func (l *TestListener) OnTimeSync()           {}
func (l *TestListener) OnInternalError(error) {}
func (l *TestListener) OnConnected(_ string)  {}

type DummyAdv struct {
	Address    ble.Addr
	Rssi       int
	NonService bool
}

type DummyAddr struct {
	Address string
}

func (addr DummyAddr) String() string { return addr.Address }

func (a DummyAdv) LocalName() string              { return "" }
func (a DummyAdv) ManufacturerData() []byte       { return nil }
func (a DummyAdv) ServiceData() []ble.ServiceData { return nil }
func (a DummyAdv) Services() []ble.UUID {
	if a.NonService {
		return nil
	}
	return GetTestServiceUUIDs()
}
func (a DummyAdv) OverflowService() []ble.UUID  { return nil }
func (a DummyAdv) TxPowerLevel() int            { return 0 }
func (a DummyAdv) Connectable() bool            { return true }
func (a DummyAdv) SolicitedService() []ble.UUID { return nil }
func (a DummyAdv) RSSI() int                    { return a.Rssi }
func (a DummyAdv) Addr() ble.Addr               { return a.Address }

func GetTestServiceUUIDs() []ble.UUID {
	u, _ := ble.Parse(util.MainServiceUUID)
	return []ble.UUID{u}
}

func GetTestServices(charUUIDs []string) []*ble.Service {
	u, _ := ble.Parse(util.MainServiceUUID)
	chars := []*ble.Characteristic{}
	for _, uuid := range charUUIDs {
		c := &ble.Characteristic{}
		uid, _ := ble.Parse(uuid)
		c.UUID = uid
		chars = append(chars, c)
	}
	return []*ble.Service{&ble.Service{u, chars, 0, 0}}
}

type TestConnection struct {
	srcAddr          string
	connectedAddr    string
	toConnectAddr    string
	rssiMap          *models.RssiMap
	mockedReadValue  map[string]*bytes.Buffer
	mockedWriteValue map[string]*bytes.Buffer
}

func NewTestConnection(addr string, toConnectAddr string, rm *models.RssiMap) *TestConnection {
	return &TestConnection{toConnectAddr: toConnectAddr, srcAddr: addr, rssiMap: rm, mockedReadValue: map[string]*bytes.Buffer{}, mockedWriteValue: map[string]*bytes.Buffer{}}
}

func (c *TestConnection) SetToConnectAddr(addr string) { c.toConnectAddr = addr }

func (c *TestConnection) SetConnectedAddr(addr string) { c.connectedAddr = addr }

func (c *TestConnection) GetMockedReadBuffer(uuid string) *bytes.Buffer {
	if val, ok := c.mockedReadValue[uuid]; ok {
		return val
	}
	buff := bytes.NewBuffer([]byte{})
	c.mockedReadValue[uuid] = buff
	return buff
}

func (c *TestConnection) GetMockedWriteBufferData(uuid string) []byte {
	return c.mockedWriteValue[uuid].Bytes()
}

func (c *TestConnection) GetConnectedAddr() string    { return c.connectedAddr }
func (c *TestConnection) GetRssiMap() *models.RssiMap { return c.rssiMap }
func (c *TestConnection) Connect(ble.AdvFilter) {
	c.connectedAddr = c.toConnectAddr
}
func (c *TestConnection) Disconnect() error {
	c.connectedAddr = ""
	return nil
}
func (c *TestConnection) Dial(a string) {
	c.connectedAddr = a
}
func (c *TestConnection) ScanForDuration(time.Duration, func(ble.Advertisement)) error {
	return nil
}
func (c *TestConnection) CollectAdvs(time.Duration) ([]ble.Advertisement, error) {
	advs := []ble.Advertisement{}
	for addr, rssi := range c.rssiMap.GetAll()[c.srcAddr] {
		advs = append(advs, DummyAdv{DummyAddr{addr}, rssi, false})
	}
	return advs, nil
}
func (c *TestConnection) Scan(fn func(ble.Advertisement)) error {
	advs, _ := c.CollectAdvs(time.Microsecond)
	for _, adv := range advs {
		fn(adv)
	}
	return nil
}
func (c *TestConnection) ReadValue(uuid string) ([]byte, error) {
	val, ok := c.mockedReadValue[uuid]
	if ok {
		return val.Bytes(), nil
	}
	return nil, errors.New("UUID not in mocked read value: " + uuid)
}
func (c *TestConnection) BlockingWriteValue(uuid string, data []byte) error {
	c.mockedWriteValue[uuid] = bytes.NewBuffer(data)
	return nil
}

func (c *TestConnection) NonBlockingWriteValue(uuid string, data []byte) {
	go func() {
		c.BlockingWriteValue(uuid, data)
	}()
}
