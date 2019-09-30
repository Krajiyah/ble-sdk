package client

import (
	"strconv"
	"testing"

	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/server"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/currantlabs/ble"
	"gotest.tools/assert"
)

const (
	testAddr       = "11:22:33:44:55:66"
	testServerAddr = "22:22:33:44:55:66"
	testSecret     = "test123"
)

var (
	attempts       = 0
	rssi           = 0
	isDisconnected = false
	mockReadChar   = []byte{}
	writeCharData  = [][]byte{}
)

func beforeEach() {
	mockReadChar = []byte{}
	writeCharData = [][]byte{}
}

type dummyClient struct{}

func (c dummyClient) ReadCharacteristic(char *ble.Characteristic) ([]byte, error) {
	return mockReadChar, nil
}
func (c dummyClient) WriteCharacteristic(char *ble.Characteristic, value []byte, noRsp bool) error {
	writeCharData = append(writeCharData, value)
	return nil
}
func (c dummyClient) Address() ble.Addr                                          { return ble.NewAddr(testAddr) }
func (c dummyClient) Name() string                                               { return "some name" }
func (c dummyClient) Profile() *ble.Profile                                      { return nil }
func (c dummyClient) DiscoverProfile(force bool) (*ble.Profile, error)           { return nil, nil }
func (c dummyClient) DiscoverServices(filter []ble.UUID) ([]*ble.Service, error) { return nil, nil }
func (c dummyClient) DiscoverIncludedServices(filter []ble.UUID, s *ble.Service) ([]*ble.Service, error) {
	return nil, nil
}
func (c dummyClient) DiscoverCharacteristics(filter []ble.UUID, s *ble.Service) ([]*ble.Characteristic, error) {
	return nil, nil
}
func (c dummyClient) DiscoverDescriptors(filter []ble.UUID, char *ble.Characteristic) ([]*ble.Descriptor, error) {
	return nil, nil
}
func (c dummyClient) ReadLongCharacteristic(char *ble.Characteristic) ([]byte, error) { return nil, nil }
func (c dummyClient) ReadDescriptor(d *ble.Descriptor) ([]byte, error)                { return nil, nil }
func (c dummyClient) WriteDescriptor(d *ble.Descriptor, v []byte) error               { return nil }
func (c dummyClient) ReadRSSI() int                                                   { return 0 }
func (c dummyClient) ExchangeMTU(rxMTU int) (txMTU int, err error)                    { return 0, nil }
func (c dummyClient) Subscribe(char *ble.Characteristic, ind bool, h ble.NotificationHandler) error {
	return nil
}
func (c dummyClient) Unsubscribe(char *ble.Characteristic, ind bool) error { return nil }
func (c dummyClient) ClearSubscriptions() error                            { return nil }
func (c dummyClient) CancelConnection() error                              { return nil }
func (c dummyClient) Disconnected() <-chan struct{}                        { return nil }

func dummyOnConnected(a int, r int) {
	attempts = a
	rssi = r
	isDisconnected = false
}

func dummyOnDisconnected() {
	isDisconnected = true
}

func setCharacteristic(client *BLEClient, uuid string) {
	u, err := ble.Parse(uuid)
	if err != nil {
		panic("could not mock read characteristic")
	}
	client.characteristics[uuid] = ble.NewCharacteristic(u)
}

func mockConnection(client *BLEClient) {
	var c ble.Client
	c = dummyClient{}
	client.cln = &c
}

func getDummyClient() *BLEClient {
	return &BLEClient{
		testAddr, testSecret, Disconnected, 0, nil, testServerAddr, map[string]int{}, makeINFContext(), nil,
		map[string]*ble.Characteristic{}, util.NewPacketAggregator(), dummyOnConnected, dummyOnDisconnected,
	}
}

func TestUnixTS(t *testing.T) {
	beforeEach()

	// mock server read
	pa := util.NewPacketAggregator()
	expected := util.UnixTS()
	encData, err := util.Encrypt([]byte(strconv.Itoa(int(expected))), testSecret)
	assert.NilError(t, err)
	guid, err := pa.AddData(encData)
	assert.NilError(t, err)
	var isLastPacket bool
	mockReadChar, isLastPacket, err = pa.PopPacketDataFromStream(guid)
	assert.NilError(t, err)
	assert.Assert(t, isLastPacket)

	// test client read
	client := getDummyClient()
	setCharacteristic(client, server.TimeSyncUUID)
	mockConnection(client)
	ts, err := client.getUnixTS()
	assert.NilError(t, err)
	assert.Equal(t, ts, expected)
}

func TestLog(t *testing.T) {
	beforeEach()

	// test client write
	client := getDummyClient()
	setCharacteristic(client, server.ClientLogUUID)
	mockConnection(client)
	expected := ClientLogRequest{"SomeAddress", Info, "Some Message"}
	err := client.Log(expected)
	assert.NilError(t, err)

	// mock write to server
	data, err := expected.Data()
	assert.NilError(t, err)
	encData, err := util.Encrypt(data, client.secret)
	assert.NilError(t, err)
	pa := util.NewPacketAggregator()
	guid1, err := pa.AddPacketFromPacketBytes(writeCharData[0])
	assert.NilError(t, err)
	guid2, err := pa.AddPacketFromPacketBytes(writeCharData[1])
	assert.NilError(t, err)
	guid3, err := pa.AddPacketFromPacketBytes(writeCharData[2])
	assert.NilError(t, err)
	assert.Equal(t, guid1, guid2)
	assert.Equal(t, guid1, guid3)
	ok := pa.HasDataFromPacketStream(guid1)
	assert.Assert(t, ok)
	encData, err = pa.PopAllDataFromPackets(guid1)
	assert.NilError(t, err)
	data, err = util.Decrypt(encData, client.secret)
	assert.NilError(t, err)
	actual, err := GetClientLogRequestFromBytes(data)
	assert.NilError(t, err)
	assert.DeepEqual(t, *actual, expected)
}
