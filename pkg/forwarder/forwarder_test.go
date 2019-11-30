package forwarder

import (
	"testing"

	"github.com/Krajiyah/ble-sdk/pkg/server"

	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/currantlabs/ble"
	"gotest.tools/assert"
)

const (
	testServerName = "Some Name"
	testAddr       = "11:22:33:44:55:66"
	testSecret     = "passwd123"
	testServerAddr = "22:22:33:44:55:66"
)

var (
	nextHop           = make(chan string)
	mockedReadValue   = []byte{}
	mockedWriteBuffer = [][]byte{}
)

type dummyListener struct{}

func (l dummyListener) OnNextHopChanged(addr string) { nextHop <- addr }
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
	dummyRssiMap RssiMap
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
	return mockedReadValue, nil
}

func (c dummyClient) RawConnect(ble.AdvFilter) error { return nil }

func (c dummyClient) WriteValue(char string, data []byte) error {
	mockedWriteBuffer = append(mockedWriteBuffer, data)
	return nil
}

type dummyServer struct{}

func (s dummyServer) Run() error { return nil }

func getDummyForwarder(t *testing.T, rssiMap RssiMap) *BLEForwarder {
	f := newBLEForwarder(testAddr, testServerAddr, dummyListener{})
	f.forwardingClient = dummyClient{rssiMap}
	f.forwardingServer = dummyServer{}
	return f
}

func TestScanLoopAndRefreshLoop(t *testing.T) {
	expectedRssiMap := RssiMap{
		testAddr: map[string]int{
			testServerAddr:      -90,
			"33:22:33:44:55:66": -10000,
		},
	}
	forwarder := getDummyForwarder(t, expectedRssiMap)
	forwarder.Run()
	addr := <-nextHop
	assert.Equal(t, addr, forwarder.serverAddr)
	assert.DeepEqual(t, forwarder.rssiMap, expectedRssiMap)
	assert.Equal(t, forwarder.toConnectAddr, forwarder.serverAddr)
	assert.Equal(t, forwarder.connectedAddr, forwarder.serverAddr)
}
