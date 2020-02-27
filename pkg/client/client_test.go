package client

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	. "github.com/Krajiyah/ble-sdk/internal"
	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"gotest.tools/assert"
)

const (
	testAddr          = "11:22:33:44:55:66"
	testServerAddr    = "22:22:33:44:55:66"
	testForwarderAddr = "33:22:33:44:55:66"
	testSecret        = "test123"
	testRSSI          = -50
)

func setServerConnection() *TestConnection {
	rm := NewRssiMap()
	rm.Set(testAddr, testServerAddr, testRSSI)
	return NewTestConnection(testAddr, testServerAddr, rm)
}

func setForwarderConnection() *TestConnection {
	rm := NewRssiMap()
	rm.Set(testAddr, testForwarderAddr, testRSSI)
	return NewTestConnection(testAddr, testForwarderAddr, rm)
}

func getTestClient(c *TestConnection) (*BLEClient, *TestListener) {
	l := &TestListener{}
	client := newBLEClient("some name", testAddr, testSecret, testServerAddr, l)
	client.connection = c
	return client, l
}

func TestHasMainService(t *testing.T) {
	assert.Equal(t, HasMainService(DummyAdv{DummyAddr{testServerAddr}, testRSSI, false}), true)
}

func mockUnixTS(buffer *bytes.Buffer) int64 {
	expected := util.UnixTS()
	buffer.Write([]byte(strconv.Itoa(int(expected))))
	return expected
}

func TestUnixTS(t *testing.T) {
	connection := setServerConnection()
	client, _ := getTestClient(connection)

	// mock server read
	expected := mockUnixTS(connection.GetMockedReadBuffer(util.TimeSyncUUID))

	// test client read
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
	connection := setServerConnection()
	client, _ := getTestClient(connection)

	// test client write
	expected := ClientLogRequest{"SomeAddress", Info, "Some Message"}
	err := client.Log(expected)
	assert.NilError(t, err)

	// mock write to server
	data := connection.GetMockedWriteBufferData(util.ClientLogUUID)
	actual, err := GetClientLogRequestFromBytes(data)
	assert.NilError(t, err)
	assert.DeepEqual(t, *actual, expected)
}

func TestConnectLoop(t *testing.T) {
	connection := setServerConnection()
	client, listener := getTestClient(connection)
	client.connectLoop()
	assert.Equal(t, listener.Attempts, 1)
	assert.Equal(t, listener.Rssi, testRSSI)
	assert.DeepEqual(t, client.connection.GetConnectedAddr(), client.serverAddr)
}

func TestScanLoop(t *testing.T) {
	connection := setServerConnection()
	client, _ := getTestClient(connection)
	go client.scanLoop()
	time.Sleep(ScanInterval + (ScanInterval / 2))
	assert.DeepEqual(t, client.connection.GetRssiMap().GetAll(), connection.GetRssiMap().GetAll())
}

func TestRun(t *testing.T) {
	connection := setServerConnection()
	client, _ := getTestClient(connection)
	client.Run()
	time.Sleep(afterConnectionDelay)
	ts := mockUnixTS(connection.GetMockedReadBuffer(util.TimeSyncUUID))
	time.Sleep(PingInterval + (PingInterval / 4))
	actual, err := client.UnixTS()
	assert.NilError(t, err)
	assert.Assert(t, actual > ts, "UnixTS must be after mocked TS")
}

func TestForwardedWrite(t *testing.T) {
	connection := setForwarderConnection()
	connection.SetConnectedAddr(testForwarderAddr)
	client, _ := getTestClient(connection)

	// 1st forward request
	req := ClientLogRequest{testAddr, Info, "Hello World!"}
	expectedData, err := req.Data()
	assert.NilError(t, err)
	err = client.Log(req)
	assert.NilError(t, err)
	data := connection.GetMockedWriteBufferData(util.WriteForwardCharUUID)
	r, err := GetForwarderRequestFromBytes(data)
	assert.NilError(t, err)
	assert.DeepEqual(t, r.Payload, expectedData)

	// 2nd forwarder request
	err = client.WriteValue(util.WriteForwardCharUUID, data)
	assert.NilError(t, err)
	data = connection.GetMockedWriteBufferData(util.WriteForwardCharUUID)
	r2, err := GetForwarderRequestFromBytes(data)
	assert.NilError(t, err)
	assert.DeepEqual(t, r, r2)
}

func TestForwardedRead(t *testing.T) {
	connection := setForwarderConnection()
	connection.SetConnectedAddr(testForwarderAddr)
	client, _ := getTestClient(connection)

	// mock forwarder read
	expected := mockUnixTS(connection.GetMockedReadBuffer(util.TimeSyncUUID))
	mockUnixTS(connection.GetMockedReadBuffer(util.EndReadForwardCharUUID))
	ts, err := client.getUnixTS()
	assert.NilError(t, err)
	assert.Equal(t, ts, expected)

	// check that it started read before doing the end read
	data := connection.GetMockedWriteBufferData(util.StartReadForwardCharUUID)
	r, err := GetForwarderRequestFromBytes(data)
	assert.NilError(t, err)
	assert.DeepEqual(t, *r, ForwarderRequest{util.TimeSyncUUID, nil, true, false})
}
