package forwarder

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/server"

	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/currantlabs/ble"
	"gotest.tools/assert"
)

const (
	invalidAddr    = "33:22:33:44:55:66"
	testServerName = "Some Name"
	testAddr       = "11:22:33:44:55:66"
	testAddr2      = "44:22:33:44:55:66"
	testSecret     = "passwd123"
	testServerAddr = "22:22:33:44:55:66"
)

type dummyListener struct{}

func (l dummyListener) OnConnectionError(err error)  {}
func (l dummyListener) OnReadOrWriteError(err error) {}
func (l dummyListener) OnError(err error)            {}

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
func (a dummyAdv) Services() []ble.UUID {
	u, _ := ble.Parse(server.MainServiceUUID)
	return []ble.UUID{u}
}
func (a dummyAdv) OverflowService() []ble.UUID  { return nil }
func (a dummyAdv) TxPowerLevel() int            { return 0 }
func (a dummyAdv) Connectable() bool            { return true }
func (a dummyAdv) SolicitedService() []ble.UUID { return nil }
func (a dummyAdv) RSSI() int                    { return a.rssi }
func (a dummyAdv) Address() ble.Addr            { return a.addr }

type dummyClient struct {
	addr              string
	dummyRssiMap      RssiMap
	mockedReadValue   *bytes.Buffer
	mockedWriteBuffer *[]*bytes.Buffer
}

func (c dummyClient) RawScan(f func(ble.Advertisement)) error {
	for k, v := range c.dummyRssiMap[c.addr] {
		f(dummyAdv{dummyAddr{k}, v})
	}
	return nil
}

func (c dummyClient) ReadValue(string) ([]byte, error) {
	return c.mockedReadValue.Bytes(), nil
}

func (c dummyClient) RawConnect(ble.AdvFilter) error { return nil }

func (c dummyClient) WriteValue(char string, data []byte) error {
	buf := bytes.NewBuffer(data)
	*c.mockedWriteBuffer = append(*c.mockedWriteBuffer, buf)
	return nil
}

type dummyServer struct{}

func (s dummyServer) Run() error { return nil }

type testStructs struct {
	forwarder         *BLEForwarder
	mockedReadValue   *bytes.Buffer
	mockedWriteBuffer *[]*bytes.Buffer
}

func getDummyForwarder(t *testing.T, addr string, rssiMap RssiMap) *testStructs {
	mockedReadValue := bytes.NewBuffer([]byte{})
	mockedWriteBuffer := &[]*bytes.Buffer{}
	f := newBLEForwarder(addr, testServerAddr, dummyListener{})
	f.forwardingClient = dummyClient{addr, rssiMap, mockedReadValue, mockedWriteBuffer}
	f.forwardingServer = dummyServer{}
	return &testStructs{f, mockedReadValue, mockedWriteBuffer}
}

func TestSingleForwarder(t *testing.T) {
	mutex := &sync.Mutex{}
	expectedRssiMap := RssiMap{
		testAddr: map[string]int{
			testServerAddr: -90,
			invalidAddr:    -10000,
		},
	}
	s := getDummyForwarder(t, testAddr, expectedRssiMap)
	forwarder := s.forwarder
	scan(forwarder, mutex, expectedRssiMap, testAddr)
	assert.DeepEqual(t, *forwarder.rssiMap, expectedRssiMap)
	assert.Equal(t, forwarder.toConnectAddr, forwarder.serverAddr)
	assert.Equal(t, forwarder.connectedAddr, forwarder.serverAddr)
}

func mockReadBuffer(t *testing.T, rssiMap *RssiMap, buffer *bytes.Buffer) {
	p, err := rssiMap.Data()
	assert.NilError(t, err)
	buffer.Write(p)
}

func scan(f *BLEForwarder, mutex *sync.Mutex, rssiMap RssiMap, addr string) {
	for k, v := range rssiMap[addr] {
		f.onScanned(dummyAdv{dummyAddr{k}, v})
	}
}

func TestDoubleForwarder(t *testing.T) {
	mutex := &sync.Mutex{}
	expectedRssiMap := RssiMap{
		testAddr: map[string]int{
			testServerAddr: -90,
			testAddr2:      -30,
		},
		testAddr2: map[string]int{
			testAddr:       -5,
			testServerAddr: -10,
		},
	}
	s1 := getDummyForwarder(t, testAddr, expectedRssiMap)
	s2 := getDummyForwarder(t, testAddr2, expectedRssiMap)
	f1, mockedReadValue1 := s1.forwarder, s1.mockedReadValue
	f2, mockedReadValue2 := s2.forwarder, s2.mockedReadValue
	scan(f1, mutex, expectedRssiMap, testAddr)
	scan(f2, mutex, expectedRssiMap, testAddr2)
	mockReadBuffer(t, f1.rssiMap, mockedReadValue2)
	mockReadBuffer(t, f2.rssiMap, mockedReadValue1)
	scan(f1, mutex, expectedRssiMap, testAddr)
	scan(f2, mutex, expectedRssiMap, testAddr2)
	assert.DeepEqual(t, *f1.rssiMap, expectedRssiMap)
	assert.DeepEqual(t, *f2.rssiMap, expectedRssiMap)
	assert.Equal(t, f1.toConnectAddr, testAddr2)
	assert.Equal(t, f1.connectedAddr, testAddr2)
	assert.Equal(t, f2.toConnectAddr, f2.serverAddr)
	assert.Equal(t, f2.connectedAddr, f2.serverAddr)
}

func TestRssiMapChar(t *testing.T) {
	rm := RssiMap{
		testAddr: map[string]int{
			testServerAddr: -90,
			invalidAddr:    -10000,
		},
	}
	s := getDummyForwarder(t, testAddr, rm)
	s.forwarder.Run()
	time.Sleep(scanInterval + (scanInterval / 2))
	assert.DeepEqual(t, *s.forwarder.rssiMap, rm)
	readChars, _ := getChars(s.forwarder)
	char := readChars[1]
	data, err := char.HandleRead(testAddr2, context.Background())
	assert.NilError(t, err)
	actualRM, err := models.GetRssiMapFromBytes(data)
	assert.NilError(t, err)
	assert.DeepEqual(t, rm, *actualRM)
}

func TestWriteChar(t *testing.T) {
	expectedRssiMap := RssiMap{
		testAddr: map[string]int{
			testServerAddr: -90,
			testAddr2:      -30,
		},
		testAddr2: map[string]int{
			testAddr:       -5,
			testServerAddr: -10,
		},
	}
	s1 := getDummyForwarder(t, testAddr, expectedRssiMap)
	s2 := getDummyForwarder(t, testAddr2, expectedRssiMap)
	f1, mockedReadValue1, mockedWriteBuffer1 := s1.forwarder, s1.mockedReadValue, s1.mockedWriteBuffer
	f2, mockedReadValue2, mockedWriteBuffer2 := s2.forwarder, s2.mockedReadValue, s2.mockedWriteBuffer
	mockReadBuffer(t, &expectedRssiMap, mockedReadValue2)
	mockReadBuffer(t, &expectedRssiMap, mockedReadValue1)
	f1.Run()
	f2.Run()
	time.Sleep(scanInterval * 2)

	// mimic client write to forwarder
	log := models.ClientLogRequest{invalidAddr, Info, "Hello World!"}
	logData, err := log.Data()
	assert.NilError(t, err)
	req := models.ForwarderRequest{server.ClientLogUUID, logData, false, true}
	data, err := req.Data()
	assert.NilError(t, err)
	_, writeChars1 := getChars(f1)
	char1 := writeChars1[0]
	char1.HandleWrite(invalidAddr, data, nil)
	assert.Equal(t, len(*mockedWriteBuffer1), 1)
	assert.DeepEqual(t, (*mockedWriteBuffer1)[0].Bytes(), data)

	// mimic 2nd forwarder passing on data to server and unpacking forwarder request
	_, writeChars2 := getChars(f2)
	char2 := writeChars2[0]
	char2.HandleWrite(testAddr, data, nil)
	assert.Equal(t, len(*mockedWriteBuffer2), 1)
	assert.DeepEqual(t, (*mockedWriteBuffer2)[0].Bytes(), logData)
}
