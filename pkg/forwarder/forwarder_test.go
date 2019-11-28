package forwarder

import (
	"testing"
	"time"

	"github.com/Krajiyah/ble-sdk/internal"
	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"gotest.tools/assert"
)

const (
	testServerName = "Some Name"
	testAddr       = "11:22:33:44:55:66"
	testSecret     = "passwd123"
	testServerAddr = "22:22:33:44:55:66"
)

type dummyListener struct{}

func (l dummyListener) OnConnectionError(err error)  {}
func (l dummyListener) OnReadOrWriteError(err error) {}
func (l dummyListener) OnError(err error)            {}

func getDummyForwarder(t *testing.T, rssiMap RssiMap) *BLEForwarder {
	f := &BLEForwarder{
		testAddr, nil, nil,
		testServerAddr, "", "", RssiMap{},
		make(chan string),
		dummyListener{},
	}
	c := internal.NewDummyCoreClient(testAddr)
	var err error
	// f.forwardingClient, err = client.NewBLEClientSharedDevice(d, testAddr, testSecret, testServerAddr, dummyClientOnConnected, dummyClientOnDisconnected)
	assert.NilError(t, err)
	// f.forwardingServer, err = server.NewBLEServerSharedDevice(d, testServerName, testSecret, dummyServerListener{}, nil, nil)
	assert.NilError(t, err)
	return f
}

func TestScanAndUpdateLoops(t *testing.T) {
	expectedRssiMap := RssiMap{
		testAddr: map[string]int{
			testServerAddr: -90,
		},
	}
	forwarder := getDummyForwarder(t, expectedRssiMap)
	forwarder.scanAndUpdateLoops()
	time.Sleep(scanInterval)
	time.Sleep(shortestPathRefreshInterval)
	// select {}
	assert.Equal(t, forwarder.rssiMap, expectedRssiMap)
	assert.Equal(t, forwarder.toConnectAddr, forwarder.serverAddr)
	assert.Equal(t, forwarder.connectedAddr, forwarder.serverAddr)
}
