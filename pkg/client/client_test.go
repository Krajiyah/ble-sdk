package client

import (
	"bytes"
	"strconv"
	"testing"

	. "github.com/Krajiyah/ble-sdk/internal"
	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/currantlabs/ble"
	"golang.org/x/net/context"
	"gotest.tools/assert"
)

const (
	testAddr       = "11:22:33:44:55:66"
	testServerAddr = "22:22:33:44:55:66"
	testSecret     = "test123"
	testRSSI       = -50
)

var (
	attempts     = 0
	rssi         = 0
	serviceUUIDs = []string{util.ClientStateUUID, util.TimeSyncUUID, util.ClientLogUUID}
	testRssiMap  = &RssiMap{
		testAddr: map[string]int{
			testServerAddr: testRSSI,
		},
	}
)

type dummyCoreClient struct {
	testAddr            string
	mockedReadCharData  *bytes.Buffer
	mockedWriteCharData *[]*bytes.Buffer
}

func newDummyCoreClient(addr string) ble.Client {
	return dummyCoreClient{addr, bytes.NewBuffer([]byte{}), &[]*bytes.Buffer{}}
}

func (c dummyCoreClient) ReadCharacteristic(char *ble.Characteristic) ([]byte, error) {
	return c.mockedReadCharData.Bytes(), nil
}
func (c dummyCoreClient) WriteCharacteristic(char *ble.Characteristic, value []byte, noRsp bool) error {
	buf := bytes.NewBuffer(value)
	*c.mockedWriteCharData = append(*c.mockedWriteCharData, buf)
	return nil
}
func (c dummyCoreClient) Address() ble.Addr { return ble.NewAddr(c.testAddr) }
func (c dummyCoreClient) Name() string      { return "some name" }
func (c dummyCoreClient) Profile() *ble.Profile {
	return &ble.Profile{
		Services: GetTestServices(serviceUUIDs),
	}
}
func (c dummyCoreClient) DiscoverProfile(force bool) (*ble.Profile, error) {
	return &ble.Profile{
		Services: GetTestServices(serviceUUIDs),
	}, nil
}
func (c dummyCoreClient) DiscoverServices(filter []ble.UUID) ([]*ble.Service, error) {
	return GetTestServices(serviceUUIDs), nil
}
func (c dummyCoreClient) DiscoverIncludedServices(filter []ble.UUID, s *ble.Service) ([]*ble.Service, error) {
	return GetTestServices(serviceUUIDs), nil
}
func (c dummyCoreClient) DiscoverCharacteristics(filter []ble.UUID, s *ble.Service) ([]*ble.Characteristic, error) {
	return nil, nil
}
func (c dummyCoreClient) DiscoverDescriptors(filter []ble.UUID, char *ble.Characteristic) ([]*ble.Descriptor, error) {
	return nil, nil
}
func (c dummyCoreClient) ReadLongCharacteristic(char *ble.Characteristic) ([]byte, error) {
	return nil, nil
}
func (c dummyCoreClient) ReadDescriptor(d *ble.Descriptor) ([]byte, error)  { return nil, nil }
func (c dummyCoreClient) WriteDescriptor(d *ble.Descriptor, v []byte) error { return nil }
func (c dummyCoreClient) ReadRSSI() int                                     { return testRSSI }
func (c dummyCoreClient) ExchangeMTU(rxMTU int) (txMTU int, err error)      { return util.MTU, nil }
func (c dummyCoreClient) Subscribe(char *ble.Characteristic, ind bool, h ble.NotificationHandler) error {
	return nil
}
func (c dummyCoreClient) Unsubscribe(char *ble.Characteristic, ind bool) error { return nil }
func (c dummyCoreClient) ClearSubscriptions() error                            { return nil }
func (c dummyCoreClient) CancelConnection() error                              { return nil }
func (c dummyCoreClient) Disconnected() <-chan struct{}                        { return nil }

type testBleConnector struct {
	addr    string
	rssiMap *RssiMap
}

func (bc testBleConnector) filter(fn func(addr string, rssi int)) {
	for k, v := range (*bc.rssiMap)[bc.addr] {
		fn(k, v)
	}
}

func (bc testBleConnector) Connect(_ context.Context, f ble.AdvFilter) (ble.Client, error) {
	bc.filter(func(addr string, rssi int) { f(DummyAdv{DummyAddr{addr}, rssi}) })
	return newDummyCoreClient(testAddr), nil
}

func (bc testBleConnector) Scan(_ context.Context, _ bool, h ble.AdvHandler, _ ble.AdvFilter) error {
	bc.filter(func(addr string, rssi int) { h(DummyAdv{DummyAddr{addr}, rssi}) })
	return nil
}

func setCharacteristic(client *BLEClient, uuid string) {
	u, err := ble.Parse(uuid)
	if err != nil {
		panic("could not mock read characteristic")
	}
	client.characteristics[uuid] = ble.NewCharacteristic(u)
}

func dummyOnConnected(a int, r int) {
	attempts = a
	rssi = r
}

func dummyOnDisconnected() {}

func getTestClient() *BLEClient {
	client := newBLEClient(testAddr, testSecret, testServerAddr, dummyOnConnected, dummyOnDisconnected)
	client.bleConnector = testBleConnector{testAddr, testRssiMap}
	return client
}

func getDummyClient() (*BLEClient, *dummyCoreClient) {
	client := getTestClient()
	cc := newDummyCoreClient(testAddr)
	var c ble.Client
	c = cc
	client.cln = &c
	dc := cc.(dummyCoreClient)
	return client, &dc
}

func TestIsForwarder(t *testing.T) {
	assert.Equal(t, IsForwarder(DummyAdv{DummyAddr{testServerAddr}, testRSSI}), true)
}

func TestUnixTS(t *testing.T) {
	client, dummyClient := getDummyClient()

	// mock server read
	pa := util.NewPacketAggregator()
	expected := util.UnixTS()
	encData, err := util.Encrypt([]byte(strconv.Itoa(int(expected))), testSecret)
	assert.NilError(t, err)
	guid, err := pa.AddData(encData)
	assert.NilError(t, err)
	var isLastPacket bool
	data, isLastPacket, err := pa.PopPacketDataFromStream(guid)
	dummyClient.mockedReadCharData.Write(data)
	assert.NilError(t, err)
	assert.Assert(t, isLastPacket)

	// test client read
	setCharacteristic(client, util.TimeSyncUUID)
	ts, err := client.getUnixTS()
	assert.NilError(t, err)
	assert.Equal(t, ts, expected)

	// test timesync
	x := util.NewTimeSync(ts)
	client.timeSync = &x
	assert.Equal(t, client.UnixTS(), ts)
}

func TestLog(t *testing.T) {
	client, dummyClient := getDummyClient()

	// test client write
	setCharacteristic(client, util.ClientLogUUID)
	expected := ClientLogRequest{"SomeAddress", Info, "Some Message"}
	err := client.Log(expected)
	assert.NilError(t, err)

	// mock write to server
	data, err := expected.Data()
	assert.NilError(t, err)
	encData, err := util.Encrypt(data, client.secret)
	assert.NilError(t, err)
	pa := util.NewPacketAggregator()
	guid1, err := pa.AddPacketFromPacketBytes((*(dummyClient.mockedWriteCharData))[0].Bytes())
	assert.NilError(t, err)
	guid2, err := pa.AddPacketFromPacketBytes((*(dummyClient.mockedWriteCharData))[1].Bytes())
	assert.NilError(t, err)
	guid3, err := pa.AddPacketFromPacketBytes((*(dummyClient.mockedWriteCharData))[2].Bytes())
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

func TestConnectLoop(t *testing.T) {
	client := getTestClient()
	client.connectLoop()
	assert.Equal(t, attempts, 1)
	assert.Equal(t, rssi, testRSSI)
}
