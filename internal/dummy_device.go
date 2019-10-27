package internal

import (
	"context"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/currantlabs/ble"
)

type dummyDevice struct {
	rssiMap models.RssiMap
}

type dummyAdv struct {
	addr ble.Addr
	rssi int
}

type dummyAddr struct {
	addr string
}

func (addr dummyAddr) String() string { return addr.addr }

func (a dummyAdv) LocalName() string              { return "" }
func (a dummyAdv) ManufacturerData() []byte       { return nil }
func (a dummyAdv) ServiceData() []ble.ServiceData { return nil }
func (a dummyAdv) Services() []ble.UUID           { return nil } // TODO: ???
func (a dummyAdv) OverflowService() []ble.UUID    { return nil }
func (a dummyAdv) TxPowerLevel() int              { return 0 }
func (a dummyAdv) Connectable() bool              { return true }
func (a dummyAdv) SolicitedService() []ble.UUID   { return nil }
func (a dummyAdv) RSSI() int                      { return a.rssi }
func (a dummyAdv) Address() ble.Addr              { return a.addr }

func (d dummyDevice) AddService(svc *ble.Service) error     { return nil }
func (d dummyDevice) RemoveAllServices() error              { return nil }
func (d dummyDevice) SetServices(svcs []*ble.Service) error { return nil }
func (d dummyDevice) Stop() error                           { return nil }
func (d dummyDevice) AdvertiseNameAndServices(ctx context.Context, name string, uuids ...ble.UUID) error {
	return nil
}
func (d dummyDevice) AdvertiseMfgData(ctx context.Context, id uint16, b []byte) error { return nil }
func (d dummyDevice) AdvertiseServiceData16(ctx context.Context, id uint16, b []byte) error {
	return nil
}
func (d dummyDevice) AdvertiseIBeaconData(ctx context.Context, b []byte) error { return nil }
func (d dummyDevice) AdvertiseIBeacon(ctx context.Context, u ble.UUID, major, minor uint16, pwr int8) error {
	return nil
}
func (d dummyDevice) Dial(ctx context.Context, a ble.Addr) (ble.Client, error) { return nil, nil }

func (d dummyDevice) Scan(ctx context.Context, allowDup bool, h ble.AdvHandler) error {
	for _, val := range d.rssiMap {
		for k, v := range val {
			time.Sleep(time.Millisecond * 250)
			h(newDummyAdv(k, v))
		}
	}
	return nil
}

func newDummyAdv(addr string, rssi int) ble.Advertisement {
	var a ble.Advertisement
	a = dummyAdv{dummyAddr{addr}, rssi}
	return a
}

func NewDummyDevice(rm models.RssiMap) ble.Device {
	return dummyDevice{rm}
}
