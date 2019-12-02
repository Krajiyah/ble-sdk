package internal

import (
	"github.com/Krajiyah/ble-sdk/pkg/server"
	"github.com/currantlabs/ble"
)

type DummyAdv struct {
	Addr ble.Addr
	Rssi int
}

type DummyAddr struct {
	Addr string
}

func (addr DummyAddr) String() string { return addr.Addr }

func (a DummyAdv) LocalName() string              { return "" }
func (a DummyAdv) ManufacturerData() []byte       { return nil }
func (a DummyAdv) ServiceData() []ble.ServiceData { return nil }
func (a DummyAdv) Services() []ble.UUID {
	return GetTestServiceUUIDs()
}
func (a DummyAdv) OverflowService() []ble.UUID  { return nil }
func (a DummyAdv) TxPowerLevel() int            { return 0 }
func (a DummyAdv) Connectable() bool            { return true }
func (a DummyAdv) SolicitedService() []ble.UUID { return nil }
func (a DummyAdv) RSSI() int                    { return a.Rssi }
func (a DummyAdv) Address() ble.Addr            { return a.Addr }

func GetTestServiceUUIDs() []ble.UUID {
	u, _ := ble.Parse(server.MainServiceUUID)
	return []ble.UUID{u}
}

func GetTestServices(charUUIDs []string) []*ble.Service {
	u, _ := ble.Parse(server.MainServiceUUID)
	chars := []*ble.Characteristic{}
	for _, uuid := range charUUIDs {
		c := &ble.Characteristic{}
		uid, _ := ble.Parse(uuid)
		c.UUID = uid
		chars = append(chars, c)
	}
	return []*ble.Service{&ble.Service{u, chars, 0, 0}}
}
