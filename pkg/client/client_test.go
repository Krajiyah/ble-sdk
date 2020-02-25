package client

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	. "github.com/Krajiyah/ble-sdk/internal"
	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/currantlabs/ble"
	"golang.org/x/net/context"
	"gotest.tools/assert"
)

const (
	testAddr          = "11:22:33:44:55:66"
	testServerAddr    = "22:22:33:44:55:66"
	testForwarderAddr = "33:22:33:44:55:66"
	testSecret        = "test123"
	testRSSI          = -50
)

var (
	serviceUUIDs = []string{util.ClientStateUUID, util.TimeSyncUUID, util.ClientLogUUID}
	testRssiMap  = NewRssiMap()
)

func setServerConnection() {
	serviceUUIDs = []string{util.ClientStateUUID, util.TimeSyncUUID, util.ClientLogUUID}
	testRssiMap = NewRssiMap()
	testRssiMap.Set(testAddr, testServerAddr, testRSSI)
}

func setForwarderConnection() {
	serviceUUIDs = []string{util.WriteForwardCharUUID, util.StartReadForwardCharUUID, util.EndReadForwardCharUUID}
	testRssiMap = NewRssiMap()
	testRssiMap.Set(testAddr, testForwarderAddr, testRSSI)
}

type dummyCoreClient struct {
	testAddr            string
	mockedReadCharData  *bytes.Buffer
	mockedWriteCharData *[]*bytes.Buffer
}

func newDummyCoreClient(addr string) ble.Client {
	return &dummyCoreClient{addr, bytes.NewBuffer([]byte{}), &[]*bytes.Buffer{}}
}

func (c *dummyCoreClient) ReadCharacteristic(char *ble.Characteristic) ([]byte, error) {
	return c.mockedReadCharData.Bytes(), nil
}
func (c *dummyCoreClient) WriteCharacteristic(char *ble.Characteristic, value []byte, noRsp bool) error {
	buf := bytes.NewBuffer(value)
	*c.mockedWriteCharData = append(*c.mockedWriteCharData, buf)
	return nil
}
func (c *dummyCoreClient) Address() ble.Addr { return ble.NewAddr(c.testAddr) }
func (c *dummyCoreClient) Name() string      { return "some name" }
func (c *dummyCoreClient) Profile() *ble.Profile {
	return &ble.Profile{
		Services: GetTestServices(serviceUUIDs),
	}
}
func (c *dummyCoreClient) DiscoverProfile(force bool) (*ble.Profile, error) {
	return &ble.Profile{
		Services: GetTestServices(serviceUUIDs),
	}, nil
}
func (c *dummyCoreClient) DiscoverServices(filter []ble.UUID) ([]*ble.Service, error) {
	return GetTestServices(serviceUUIDs), nil
}
func (c *dummyCoreClient) DiscoverIncludedServices(filter []ble.UUID, s *ble.Service) ([]*ble.Service, error) {
	return GetTestServices(serviceUUIDs), nil
}
func (c *dummyCoreClient) DiscoverCharacteristics(filter []ble.UUID, s *ble.Service) ([]*ble.Characteristic, error) {
	return nil, nil
}
func (c *dummyCoreClient) DiscoverDescriptors(filter []ble.UUID, char *ble.Characteristic) ([]*ble.Descriptor, error) {
	return nil, nil
}
func (c *dummyCoreClient) ReadLongCharacteristic(char *ble.Characteristic) ([]byte, error) {
	return nil, nil
}
func (c *dummyCoreClient) ReadDescriptor(d *ble.Descriptor) ([]byte, error)  { return nil, nil }
func (c *dummyCoreClient) WriteDescriptor(d *ble.Descriptor, v []byte) error { return nil }
func (c *dummyCoreClient) ReadRSSI() int                                     { return testRSSI }
func (c *dummyCoreClient) ExchangeMTU(rxMTU int) (txMTU int, err error)      { return util.MTU, nil }
func (c *dummyCoreClient) Subscribe(char *ble.Characteristic, ind bool, h ble.NotificationHandler) error {
	return nil
}
func (c *dummyCoreClient) Unsubscribe(char *ble.Characteristic, ind bool) error { return nil }
func (c *dummyCoreClient) ClearSubscriptions() error                            { return nil }
func (c *dummyCoreClient) CancelConnection() error                              { return nil }
func (c *dummyCoreClient) Disconnected() <-chan struct{}                        { return nil }

type testBleConnector struct {
	addr    string
	rssiMap *RssiMap
}

func (bc testBleConnector) filter(fn func(addr string, rssi int)) {
	for k, v := range bc.rssiMap.GetAll()[bc.addr] {
		fn(k, v)
	}
}

func (bc testBleConnector) Connect(_ context.Context, f ble.AdvFilter) (ble.Client, error) {
	bc.filter(func(addr string, rssi int) { f(DummyAdv{DummyAddr{addr}, rssi, false}) })
	return newDummyCoreClient(testAddr), nil
}

func (bc testBleConnector) Scan(_ context.Context, _ bool, h ble.AdvHandler, _ ble.AdvFilter) error {
	bc.filter(func(addr string, rssi int) { h(DummyAdv{DummyAddr{addr}, rssi, false}) })
	return nil
}

func setCharacteristic(client *BLEClient, uuid string) {
	u, err := ble.Parse(uuid)
	if err != nil {
		panic("could not mock read characteristic")
	}
	client.characteristics[uuid] = ble.NewCharacteristic(u)
}

type dummyListener struct {
	attempts int
	rssi     int
}

func (l *dummyListener) OnConnected(_ string, a int, r int) {
	l.attempts = a
	l.rssi = r
}

func (l *dummyListener) OnDisconnected() {}

func (l *dummyListener) OnTimeSync() {}

func getTestClient() (*BLEClient, *dummyListener) {
	l := &dummyListener{}
	client := newBLEClient("some name", testAddr, testSecret, testServerAddr, l)
	client.bleConnector = testBleConnector{testAddr, testRssiMap}
	return client, l
}

func getDummyClient(connectedAddr string) (*BLEClient, *dummyCoreClient, *dummyListener) {
	client, l := getTestClient()
	cc := newDummyCoreClient(testAddr)
	var c ble.Client
	c = cc
	client.cln = &c
	client.connectedAddr = connectedAddr
	client.characteristics = map[string]*ble.Characteristic{}
	for _, uuid := range serviceUUIDs {
		client.characteristics[uuid] = nil
	}
	dc := cc.(*dummyCoreClient)
	return client, dc, l
}

func TestIsForwarder(t *testing.T) {
	assert.Equal(t, IsForwarder(DummyAdv{DummyAddr{testServerAddr}, testRSSI, false}), true)
}

func mockUnixTS(t *testing.T, buffer *bytes.Buffer) int64 {
	pa := util.NewPacketAggregator()
	expected := util.UnixTS()
	encData, err := util.Encrypt([]byte(strconv.Itoa(int(expected))), testSecret)
	assert.NilError(t, err)
	guid, err := pa.AddData(encData)
	assert.NilError(t, err)
	isLastPacket := false
	for !isLastPacket {
		var err error
		var data []byte
		data, isLastPacket, err = pa.PopPacketDataFromStream(guid)
		assert.NilError(t, err)
		buffer.Write(data)
	}
	return expected
}

func getWriteBufferData(t *testing.T, secret string, buffers *[]*bytes.Buffer) []byte {
	pa := util.NewPacketAggregator()
	var guid string
	var err error
	for _, buffer := range *buffers {
		guid, err = pa.AddPacketFromPacketBytes(buffer.Bytes())
		assert.NilError(t, err)
	}
	ok := pa.HasDataFromPacketStream(guid)
	assert.Assert(t, ok)
	encData, err := pa.PopAllDataFromPackets(guid)
	assert.NilError(t, err)
	data, err := util.Decrypt(encData, secret)
	assert.NilError(t, err)
	return data
}

func TestUnixTS(t *testing.T) {
	setServerConnection()
	client, dummyClient, _ := getDummyClient(testServerAddr)

	// mock server read
	expected := mockUnixTS(t, dummyClient.mockedReadCharData)

	// test client read
	setCharacteristic(client, util.TimeSyncUUID)
	ts, err := client.getUnixTS()
	assert.NilError(t, err)
	assert.Equal(t, ts, expected)

	// test timesync
	x := util.NewTimeSync(ts)
	client.timeSync = &x
	actual, err := client.UnixTS()
	assert.NilError(t, err)
	assert.Equal(t, actual, ts)
}

func TestLog(t *testing.T) {
	setServerConnection()
	client, dummyClient, _ := getDummyClient(testServerAddr)

	// test client write
	setCharacteristic(client, util.ClientLogUUID)
	expected := ClientLogRequest{"SomeAddress", Info, "Some Message"}
	err := client.Log(expected)
	assert.NilError(t, err)

	// mock write to server
	data := getWriteBufferData(t, client.secret, dummyClient.mockedWriteCharData)
	actual, err := GetClientLogRequestFromBytes(data)
	assert.NilError(t, err)
	assert.DeepEqual(t, *actual, expected)
}

func TestConnectLoop(t *testing.T) {
	setServerConnection()
	client, listener := getTestClient()
	client.connectLoop()
	assert.Equal(t, listener.attempts, 1)
	assert.Equal(t, listener.rssi, testRSSI)
	assert.DeepEqual(t, client.GetConnectionGraph().GetAll(), NewConnectionGraphFromRaw(map[string]string{client.addr: client.serverAddr}).GetAll())
}

func TestScanLoop(t *testing.T) {
	setServerConnection()
	client, _ := getTestClient()
	go client.scan()
	time.Sleep(ScanInterval + (ScanInterval / 2))
	assert.DeepEqual(t, client.GetRssiMap().GetAll(), testRssiMap.GetAll())
}

func TestRun(t *testing.T) {
	setServerConnection()
	client, _ := getTestClient()
	client.Run()
	time.Sleep(afterConnectionDelay)
	c := (*(client.cln)).(*dummyCoreClient)
	ts := mockUnixTS(t, c.mockedReadCharData)
	time.Sleep(PingInterval + (PingInterval / 4))
	actual, err := client.UnixTS()
	assert.NilError(t, err)
	assert.Assert(t, actual > ts, "UnixTS must be after mocked TS")
}

func TestForwardedWrite(t *testing.T) {
	setForwarderConnection()

	// 1st forward request
	client, dummyCoreClient, _ := getDummyClient(testForwarderAddr)
	req := ClientLogRequest{testAddr, Info, "Hello World!"}
	expectedData, err := req.Data()
	assert.NilError(t, err)
	err = client.Log(req)
	assert.NilError(t, err)
	data := getWriteBufferData(t, client.secret, dummyCoreClient.mockedWriteCharData)
	r, err := GetForwarderRequestFromBytes(data)
	assert.NilError(t, err)
	assert.DeepEqual(t, r.Payload, expectedData)

	// 2nd forwarder request
	dummyCoreClient.mockedWriteCharData = &[]*bytes.Buffer{}
	err = client.WriteValue(util.WriteForwardCharUUID, data)
	assert.NilError(t, err)
	data = getWriteBufferData(t, client.secret, dummyCoreClient.mockedWriteCharData)
	r2, err := GetForwarderRequestFromBytes(data)
	assert.NilError(t, err)
	assert.DeepEqual(t, r, r2)
}

func TestForwardedRead(t *testing.T) {
	setForwarderConnection()
	client, dummyClient, _ := getDummyClient(testForwarderAddr)

	// mock forwarder read
	expected := mockUnixTS(t, dummyClient.mockedReadCharData)

	ts, err := client.getUnixTS()
	assert.NilError(t, err)
	assert.Equal(t, ts, expected)

	// check that it started read before doing the end read
	data := getWriteBufferData(t, client.secret, dummyClient.mockedWriteCharData)
	r, err := GetForwarderRequestFromBytes(data)
	assert.NilError(t, err)
	assert.DeepEqual(t, *r, ForwarderRequest{util.TimeSyncUUID, nil, true, false})
}
