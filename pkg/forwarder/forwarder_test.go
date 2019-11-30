package forwarder

import (
	"testing"

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

type dummyListener struct {
	nextHop chan string
}

func (l dummyListener) OnNextHopChanged(addr string) { l.nextHop <- addr }
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
	dummyRssiMap      RssiMap
	mockedReadValue   []byte
	mockedWriteBuffer [][]byte
}

func (c dummyClient) RawScan(f func(ble.Advertisement)) error {
	for _, val := range c.dummyRssiMap {
		for k, v := range val {
			f(dummyAdv{dummyAddr{k}, v})
		}
	}
	return nil
}

func (c dummyClient) ReadValue(string) ([]byte, error) {
	return c.mockedReadValue, nil
}

func (c dummyClient) RawConnect(ble.AdvFilter) error { return nil }

func (c dummyClient) WriteValue(char string, data []byte) error {
	c.mockedWriteBuffer = append(c.mockedWriteBuffer, data)
	return nil
}

type dummyServer struct{}

func (s dummyServer) Run() error { return nil }

type testStructs struct {
	forwarder         *BLEForwarder
	nextHop           chan string
	mockedReadValue   []byte
	mockedWriteBuffer [][]byte
}

func getDummyForwarder(t *testing.T, addr string, rssiMap RssiMap) testStructs {
	nextHop := make(chan string)
	mockedReadValue := []byte{}
	mockedWriteBuffer := [][]byte{}
	f := newBLEForwarder(addr, testServerAddr, dummyListener{nextHop})
	f.forwardingClient = dummyClient{rssiMap, mockedReadValue, mockedWriteBuffer}
	f.forwardingServer = dummyServer{}
	return testStructs{f, nextHop, mockedReadValue, mockedWriteBuffer}
}

func TestSingleForwarder(t *testing.T) {
	expectedRssiMap := RssiMap{
		testAddr: map[string]int{
			testServerAddr: -90,
			invalidAddr:    -10000,
		},
	}
	s := getDummyForwarder(t, testAddr, expectedRssiMap)
	forwarder, nextHop := s.forwarder, s.nextHop
	forwarder.Run()
	addr := <-nextHop
	assert.Equal(t, addr, forwarder.serverAddr)
	assert.DeepEqual(t, forwarder.rssiMap, expectedRssiMap)
	assert.Equal(t, forwarder.toConnectAddr, forwarder.serverAddr)
	assert.Equal(t, forwarder.connectedAddr, forwarder.serverAddr)
}

func TestDoubleForwarder(t *testing.T) {
	expectedRssiMap := RssiMap{
		testAddr: map[string]int{
			testServerAddr: -90,
			testAddr2:      -30,
		},
		testAddr2: map[string]int{
			testServerAddr: -10,
		},
	}
	s1 := getDummyForwarder(t, testAddr, expectedRssiMap)
	s2 := getDummyForwarder(t, testAddr2, expectedRssiMap)
	f1, nextHop1 := s1.forwarder, s1.nextHop
	f2, nextHop2 := s2.forwarder, s2.nextHop
	f1.Run()
	f2.Run()
	addr2 := <-nextHop2
	assert.Equal(t, addr2, f2.serverAddr)
	assert.DeepEqual(t, f2.rssiMap, expectedRssiMap)
	assert.Equal(t, f2.toConnectAddr, f2.serverAddr)
	assert.Equal(t, f2.connectedAddr, f2.serverAddr)
	addr1 := <-nextHop1
	assert.Equal(t, addr1, testAddr2)
	assert.DeepEqual(t, f1.rssiMap, expectedRssiMap)
	assert.Equal(t, f1.toConnectAddr, testAddr2)
	assert.Equal(t, f1.connectedAddr, testAddr2)
}
