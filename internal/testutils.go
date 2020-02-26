package internal

import (
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
)

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
