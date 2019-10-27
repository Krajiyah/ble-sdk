package internal

import "github.com/currantlabs/ble"

var (
	MockReadCharData  []byte
	MockWriteCharData [][]byte
)

type DummyCoreClient struct {
	testAddr string
}

func ResetDummyDataBuffers() {
	MockReadCharData = []byte{}
	MockWriteCharData = [][]byte{}
}

func NewDummyCoreClient(addr string) ble.Client {
	return DummyCoreClient{addr}
}

func (c DummyCoreClient) ReadCharacteristic(char *ble.Characteristic) ([]byte, error) {
	return MockReadCharData, nil
}
func (c DummyCoreClient) WriteCharacteristic(char *ble.Characteristic, value []byte, noRsp bool) error {
	MockWriteCharData = append(MockWriteCharData, value)
	return nil
}
func (c DummyCoreClient) Address() ble.Addr                                          { return ble.NewAddr(c.testAddr) }
func (c DummyCoreClient) Name() string                                               { return "some name" }
func (c DummyCoreClient) Profile() *ble.Profile                                      { return nil }
func (c DummyCoreClient) DiscoverProfile(force bool) (*ble.Profile, error)           { return nil, nil }
func (c DummyCoreClient) DiscoverServices(filter []ble.UUID) ([]*ble.Service, error) { return nil, nil }
func (c DummyCoreClient) DiscoverIncludedServices(filter []ble.UUID, s *ble.Service) ([]*ble.Service, error) {
	return nil, nil
}
func (c DummyCoreClient) DiscoverCharacteristics(filter []ble.UUID, s *ble.Service) ([]*ble.Characteristic, error) {
	return nil, nil
}
func (c DummyCoreClient) DiscoverDescriptors(filter []ble.UUID, char *ble.Characteristic) ([]*ble.Descriptor, error) {
	return nil, nil
}
func (c DummyCoreClient) ReadLongCharacteristic(char *ble.Characteristic) ([]byte, error) {
	return nil, nil
}
func (c DummyCoreClient) ReadDescriptor(d *ble.Descriptor) ([]byte, error)  { return nil, nil }
func (c DummyCoreClient) WriteDescriptor(d *ble.Descriptor, v []byte) error { return nil }
func (c DummyCoreClient) ReadRSSI() int                                     { return 0 }
func (c DummyCoreClient) ExchangeMTU(rxMTU int) (txMTU int, err error)      { return 0, nil }
func (c DummyCoreClient) Subscribe(char *ble.Characteristic, ind bool, h ble.NotificationHandler) error {
	return nil
}
func (c DummyCoreClient) Unsubscribe(char *ble.Characteristic, ind bool) error { return nil }
func (c DummyCoreClient) ClearSubscriptions() error                            { return nil }
func (c DummyCoreClient) CancelConnection() error                              { return nil }
func (c DummyCoreClient) Disconnected() <-chan struct{}                        { return nil }
