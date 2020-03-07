package ble

import (
	"bytes"
	"context"
	"testing"
	"time"

	. "github.com/Krajiyah/ble-sdk/internal"
	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
	"gotest.tools/assert"
)

const (
	testAddr      = "11:22:33:44:55:66"
	testOtherAddr = "A"
	testRSSI      = -60
	testSecret    = "passwd123"
	testCharUUID  = "00010000-0005-2000-5000-00805F9B34FB"
)

var (
	serviceUUIDs = []string{util.ClientStateUUID}
	rm           = models.NewRssiMapFromRaw(map[string]map[string]int{
		testAddr: map[string]int{
			testOtherAddr: -60,
		},
	})
)

type testCoreMethods struct{}

func (bc *testCoreMethods) filter(fn func(addr string, rssi int)) {
	for k, v := range rm.GetAll()[testAddr] {
		fn(k, v)
	}
}

func (bc *testCoreMethods) Stop() error             { return nil }
func (bc *testCoreMethods) SetDefaultDevice() error { return nil }
func (bc *testCoreMethods) AdvertiseNameAndServices(ctx context.Context, name string, uuids ...ble.UUID) error {
	return nil
}
func (bc *testCoreMethods) AddService(s *ble.Service) error { return nil }
func (bc *testCoreMethods) Dial(_ context.Context, a ble.Addr) (ble.Client, error) {
	return newDummyCoreClient(), nil
}

func (bc *testCoreMethods) Connect(_ context.Context, f ble.AdvFilter) (ble.Client, error) {
	bc.filter(func(addr string, rssi int) { f(DummyAdv{DummyAddr{addr}, rssi, false}) })
	return newDummyCoreClient(), nil
}

func (bc *testCoreMethods) Scan(_ context.Context, _ bool, h ble.AdvHandler, _ ble.AdvFilter) error {
	bc.filter(func(addr string, rssi int) { h(DummyAdv{DummyAddr{addr}, rssi, false}) })
	return nil
}

type mockConn struct {
	ctx context.Context
}

func (c *mockConn) Context() context.Context          { return c.ctx }
func (c *mockConn) SetContext(ctx context.Context)    { c.ctx = ctx }
func (c *mockConn) LocalAddr() ble.Addr               { return ble.NewAddr(testAddr) }
func (c *mockConn) RemoteAddr() ble.Addr              { return ble.NewAddr(testAddr) }
func (c *mockConn) RxMTU() int                        { return util.MTU }
func (c *mockConn) SetRxMTU(mtu int)                  {}
func (c *mockConn) TxMTU() int                        { return util.MTU }
func (c *mockConn) SetTxMTU(mtu int)                  {}
func (c *mockConn) Disconnected() <-chan struct{}     { return make(chan struct{}) }
func (c *mockConn) Read(p []byte) (n int, err error)  { return 0, nil }
func (c *mockConn) Write(p []byte) (n int, err error) { return 0, nil }
func (c *mockConn) Close() error                      { return nil }

type dummyCoreClient struct {
	testAddr            string
	mockedReadCharData  *bytes.Buffer
	mockedWriteCharData *[]*bytes.Buffer
}

func newDummyCoreClient() ble.Client {
	return &dummyCoreClient{testAddr, bytes.NewBuffer([]byte{}), &[]*bytes.Buffer{}}
}

func (c *dummyCoreClient) ReadCharacteristic(char *ble.Characteristic) ([]byte, error) {
	return c.mockedReadCharData.Bytes(), nil
}
func (c *dummyCoreClient) WriteCharacteristic(char *ble.Characteristic, value []byte, noRsp bool) error {
	buf := bytes.NewBuffer(value)
	*c.mockedWriteCharData = append(*c.mockedWriteCharData, buf)
	return nil
}
func (c *dummyCoreClient) Addr() ble.Addr { return ble.NewAddr(c.testAddr) }
func (c *dummyCoreClient) Name() string   { return "some name" }
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
	return c.ReadCharacteristic(char)
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
func (c *dummyCoreClient) Disconnected() <-chan struct{}                        { return make(chan struct{}) }
func (c *dummyCoreClient) Conn() ble.Conn                                       { return &mockConn{ctx: context.Background()} }

func setCharacteristic(c *RealConnection, uuid string) {
	u, err := ble.Parse(uuid)
	if err != nil {
		panic("could not mock read characteristic")
	}
	c.characteristics[uuid] = ble.NewCharacteristic(u)
}

func setDummyCoreClient(c *RealConnection, d ble.Client) {
	c.cln = d
}

func newConnection(t *testing.T) *RealConnection {
	c, err := newRealConnection(testAddr, testSecret, time.Minute, &TestListener{}, &testCoreMethods{}, nil)
	assert.NilError(t, err)
	return c
}

func TestConnect(t *testing.T) {
	c := newConnection(t)
	err := c.Connect(func(a ble.Advertisement) bool {
		return true
	})
	assert.NilError(t, err)
	assert.DeepEqual(t, rm.GetAll(), c.GetRssiMap().GetAll())
	assert.Equal(t, c.GetConnectedAddr(), testOtherAddr)
}

func TestScan(t *testing.T) {
	c := newConnection(t)
	actual := make(chan ble.Advertisement)
	err := c.Scan(func(a ble.Advertisement) {
		go func() { actual <- a }()
	})
	assert.NilError(t, err)
	assert.DeepEqual(t, rm.GetAll(), c.GetRssiMap().GetAll())
	adv := <-actual
	assert.Check(t, util.AddrEqualAddr(adv.Addr().String(), testOtherAddr))
	assert.Equal(t, testRSSI, adv.RSSI())
}

func TestRead(t *testing.T) {
	c := newConnection(t)
	setCharacteristic(c, testCharUUID)
	d := newDummyCoreClient()
	setDummyCoreClient(c, d)
	expected := []byte("Hello World!")
	encData, err := util.Encrypt(expected, testSecret)
	assert.NilError(t, err)
	client := d.(*dummyCoreClient)
	client.mockedReadCharData.Write(encData)
	actual, err := c.ReadValue(testCharUUID)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, actual)
}

func TestWrite(t *testing.T) {
	c := newConnection(t)
	setCharacteristic(c, testCharUUID)
	d := newDummyCoreClient()
	setDummyCoreClient(c, d)
	expected := []byte("Hello World!")
	err := c.WriteValue(testCharUUID, expected)
	assert.NilError(t, err)
	client := d.(*dummyCoreClient)
	buff := util.NewPacketBuffer(testSecret)
	var actual []byte
	for _, b := range *client.mockedWriteCharData {
		actual, err = buff.Set(b.Bytes())
		assert.NilError(t, err)
	}
	assert.DeepEqual(t, expected, actual)
}
