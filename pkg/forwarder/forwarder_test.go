package forwarder

import (
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	. "github.com/Krajiyah/ble-sdk/internal"
	"github.com/Krajiyah/ble-sdk/pkg/ble"
	"github.com/Krajiyah/ble-sdk/pkg/client"
	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/pkg/errors"

	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"gotest.tools/assert"
)

const (
	clientAddr              = "33:22:33:44:55:66"
	testServerName          = "Some Name"
	testAddr                = "11:22:33:44:55:66"
	testAddr2               = "44:22:33:44:55:66"
	testSecret              = "passwd123"
	testServerAddr          = "22:22:33:44:55:66"
	waitForNonBlockingWrite = time.Second * 3
)

type dummyListener struct{}

func (l dummyListener) OnServerStatusChanged(models.BLEServerStatus, error) {}
func (l dummyListener) OnInternalError(error)                               {}
func (l dummyListener) OnConnected(string)                                  {}
func (l dummyListener) OnDisconnected()                                     {}
func (l dummyListener) OnTimeSync()                                         {}

type dummyClient struct {
	addr              string
	connection        *TestConnection
	dummyRssiMap      *RssiMap
	mockedReadValue   map[string]*bytes.Buffer
	mockedWriteBuffer map[string]*bytes.Buffer
}

func (c *dummyClient) ReadValue(uuid string) ([]byte, error) {
	buff, ok := c.mockedReadValue[uuid]
	if ok {
		return buff.Bytes(), nil
	}
	return nil, errors.New("no buffer")
}

func (c *dummyClient) GetConnection() ble.Connection {
	return c.connection
}

func (c *dummyClient) WriteValue(char string, data []byte, block bool) error {
	buf := bytes.NewBuffer(data)
	c.mockedWriteBuffer[char] = buf
	return nil
}

func (c *dummyClient) Log(ClientLogRequest) error { return nil }
func (c *dummyClient) UnixTS() (int64, error)     { return 0, nil }
func (c *dummyClient) Run()                       {}

type dummyServer struct {
	rssiMap *RssiMap
}

func (s *dummyServer) GetRssiMap() *RssiMap                 { return s.rssiMap }
func (s *dummyServer) GetConnectionGraph() *ConnectionGraph { return NewConnectionGraph() }
func (s *dummyServer) Run() error                           { return nil }

type testStructs struct {
	forwarder         *BLEForwarder
	mockedReadValue   map[string]*bytes.Buffer
	mockedWriteBuffer map[string]*bytes.Buffer
}

func getDummyForwarder(t *testing.T, addr string, rssiMap *RssiMap) *testStructs {
	mockedReadValue := map[string]*bytes.Buffer{}
	mockedWriteBuffer := map[string]*bytes.Buffer{}
	f := newBLEForwarder("some name", addr, testServerAddr, dummyListener{})
	f.forwardingClient = &dummyClient{addr, NewTestConnection(addr, "empty", rssiMap), rssiMap, mockedReadValue, mockedWriteBuffer}
	f.forwardingServer = &dummyServer{rssiMap}
	return &testStructs{f, mockedReadValue, mockedWriteBuffer}
}

func TestSingleForwarder(t *testing.T) {
	expectedRssiMap := models.NewRssiMap()
	expectedRssiMap.Set(testAddr, testServerAddr, -90)
	expectedRssiMap.Set(testAddr, clientAddr, -10000)
	s := getDummyForwarder(t, testAddr, expectedRssiMap)
	forwarder := s.forwarder
	conn := forwarder.forwardingClient.GetConnection().(*TestConnection)
	conn.SetToConnectAddr(forwarder.serverAddr)
	scan(t, forwarder, expectedRssiMap, testAddr)
	assert.DeepEqual(t, forwarder.GetRssiMap(), expectedRssiMap.GetAll())
	assert.Equal(t, forwarder.toConnectAddr, forwarder.serverAddr)
	assert.Equal(t, conn.GetConnectedAddr(), forwarder.serverAddr)
}

func mockRMReadBuffer(t *testing.T, rssiMap *RssiMap, buffers map[string]*bytes.Buffer) {
	p, err := rssiMap.Data()
	assert.NilError(t, err)
	buffer := bytes.NewBuffer(p)
	buffers[util.ReadRssiMapCharUUID] = buffer
}

func mockCGReadBuffer(t *testing.T, cg *ConnectionGraph, buffers map[string]*bytes.Buffer) {
	p, err := cg.Data()
	assert.NilError(t, err)
	buffer := bytes.NewBuffer(p)
	buffers[util.ReadConnectionGraphUUID] = buffer
}

func scan(t *testing.T, f *BLEForwarder, rssiMap *RssiMap, addr string) {
	for k, v := range rssiMap.GetAll()[addr] {
		f.onScanned(DummyAdv{DummyAddr{k}, v, util.AddrEqualAddr(k, clientAddr)})
	}
}

func TestDoubleForwarder(t *testing.T) {
	expectedRssiMap := models.NewRssiMap()
	expectedRssiMap.Set(testAddr, testAddr2, -30)
	expectedRssiMap.Set(testAddr2, testAddr, -5)
	expectedRssiMap.Set(testAddr2, testServerAddr, -10)
	s1 := getDummyForwarder(t, testAddr, expectedRssiMap)
	s2 := getDummyForwarder(t, testAddr2, expectedRssiMap)
	f1, mockedReadValue1 := s1.forwarder, s1.mockedReadValue
	f2, mockedReadValue2 := s2.forwarder, s2.mockedReadValue
	mockRMReadBuffer(t, NewRssiMapFromRaw(f1.GetRssiMap()), mockedReadValue2)
	mockRMReadBuffer(t, NewRssiMapFromRaw(f2.GetRssiMap()), mockedReadValue1)
	mockCGReadBuffer(t, NewConnectionGraphFromRaw(f1.GetConnectionGraph()), mockedReadValue2)
	mockCGReadBuffer(t, NewConnectionGraphFromRaw(f2.GetConnectionGraph()), mockedReadValue1)
	conn1 := f1.forwardingClient.GetConnection().(*TestConnection)
	conn1.SetToConnectAddr(testAddr2)
	conn2 := f2.forwardingClient.GetConnection().(*TestConnection)
	conn2.SetToConnectAddr(testServerAddr)
	scan(t, f1, expectedRssiMap, testAddr)
	scan(t, f2, expectedRssiMap, testAddr2)
	assert.DeepEqual(t, f1.GetRssiMap(), expectedRssiMap.GetAll())
	assert.DeepEqual(t, f2.GetRssiMap(), expectedRssiMap.GetAll())
	assert.Equal(t, f1.toConnectAddr, testAddr2)
	assert.Equal(t, f1.forwardingClient.GetConnection().GetConnectedAddr(), testAddr2)
	assert.Equal(t, f2.toConnectAddr, f2.serverAddr)
	assert.Equal(t, f2.forwardingClient.GetConnection().GetConnectedAddr(), f2.serverAddr)
}

func TestRssiMapChar(t *testing.T) {
	rm := models.NewRssiMap()
	rm.Set(testAddr, testServerAddr, -90)
	rm.Set(testAddr, clientAddr, -10000)
	s := getDummyForwarder(t, testAddr, rm)
	go func() {
		s.forwarder.Run()
	}()
	time.Sleep(2 * client.ScanInterval)
	assert.DeepEqual(t, s.forwarder.GetRssiMap(), rm.GetAll())
	readChars, _ := getChars(s.forwarder)
	char := readChars[1]
	data, err := char.HandleRead(testAddr2, context.Background())
	assert.NilError(t, err)
	actualRM, err := models.GetRssiMapFromBytes(data)
	assert.NilError(t, err)
	assert.DeepEqual(t, rm.GetAll(), actualRM.GetAll())
}

func prepare2ForwarderState(t *testing.T) (*testStructs, *testStructs) {
	expectedRssiMap := models.NewRssiMap()
	expectedRssiMap.Set(testAddr, testServerAddr, -90)
	expectedRssiMap.Set(testAddr, testAddr2, -30)
	expectedRssiMap.Set(testAddr2, testServerAddr, -10)
	s1 := getDummyForwarder(t, testAddr, expectedRssiMap)
	s2 := getDummyForwarder(t, testAddr2, expectedRssiMap)
	f1, mockedReadValue1, _ := s1.forwarder, s1.mockedReadValue, s1.mockedWriteBuffer
	f2, mockedReadValue2, _ := s2.forwarder, s2.mockedReadValue, s2.mockedWriteBuffer
	mockRMReadBuffer(t, expectedRssiMap, mockedReadValue2)
	mockRMReadBuffer(t, expectedRssiMap, mockedReadValue1)
	mockCGReadBuffer(t, NewConnectionGraph(), mockedReadValue2)
	mockCGReadBuffer(t, NewConnectionGraph(), mockedReadValue1)
	go func() { f1.Run() }()
	go func() { f2.Run() }()
	time.Sleep(client.ScanInterval * 2)
	return s1, s2
}

func TestWriteChar(t *testing.T) {
	s1, s2 := prepare2ForwarderState(t)
	f1, _, mockedWriteBuffer1 := s1.forwarder, s1.mockedReadValue, s1.mockedWriteBuffer
	f2, _, mockedWriteBuffer2 := s2.forwarder, s2.mockedReadValue, s2.mockedWriteBuffer
	c1 := f1.forwardingClient.(*dummyClient)
	c1.connection.SetConnectedAddr(f2.addr)
	c2 := f2.forwardingClient.(*dummyClient)
	c2.connection.SetConnectedAddr(testServerAddr)

	// mimic client write to forwarder
	log := models.ClientLogRequest{clientAddr, Info, "Hello World!"}
	logData, err := log.Data()
	assert.NilError(t, err)
	req := models.ForwarderRequest{util.ClientLogUUID, logData, false, true}
	data, err := req.Data()
	assert.NilError(t, err)
	_, writeChars1 := getChars(f1)
	char1 := writeChars1[0]
	char1.HandleWrite(clientAddr, data, nil)
	time.Sleep(waitForNonBlockingWrite)
	bufferData1 := mockedWriteBuffer1[util.WriteForwardCharUUID].Bytes()
	assert.DeepEqual(t, bufferData1, data)

	// mimic 2nd forwarder passing on data to server and unpacking forwarder request
	_, writeChars2 := getChars(f2)
	char2 := writeChars2[0]
	char2.HandleWrite(testAddr, data, nil)
	time.Sleep(waitForNonBlockingWrite)
	bufferData2 := mockedWriteBuffer2[util.ClientLogUUID].Bytes()
	assert.DeepEqual(t, bufferData2, logData)
}

func TestStartEndReadChars(t *testing.T) {
	s1, s2 := prepare2ForwarderState(t)
	f1, mockedReadValue1, mockedWriteBuffer1 := s1.forwarder, s1.mockedReadValue, s1.mockedWriteBuffer
	f2, mockedReadValue2, _ := s2.forwarder, s2.mockedReadValue, s2.mockedWriteBuffer
	c1 := f1.forwardingClient.(*dummyClient)
	c1.connection.SetConnectedAddr(f2.addr)
	c2 := f2.forwardingClient.(*dummyClient)
	c2.connection.SetConnectedAddr(testServerAddr)

	// mimic client write to forwarder (start read request)
	req := models.ForwarderRequest{util.TimeSyncUUID, nil, true, false}
	data, err := req.Data()
	assert.NilError(t, err)
	readChars1, writeChars1 := getChars(f1)
	writeChar1 := writeChars1[1]
	writeChar1.HandleWrite(clientAddr, data, nil)
	time.Sleep(waitForNonBlockingWrite)
	bufferData1 := mockedWriteBuffer1[util.StartReadForwardCharUUID].Bytes()
	assert.Check(t, !f1.isConnectedToServer(), "F1 should not be connected to server")
	assert.DeepEqual(t, bufferData1, data)

	// mimic 2nd forwarder preparing for end read request
	readChars2, writeChars2 := getChars(f2)
	writeChar2 := writeChars2[1]
	writeChar2.HandleWrite(testAddr, data, nil)
	assert.Check(t, f2.isConnectedToServer(), "F2 should be connected to server")

	// mimic client read from forwarder (end read request)
	ts := []byte(strconv.FormatInt(util.UnixTS(), 10))
	mockedReadValue1[util.EndReadForwardCharUUID] = bytes.NewBuffer(ts)
	readChar1 := readChars1[0]
	data, err = readChar1.HandleRead(clientAddr, context.Background())
	assert.NilError(t, err)
	assert.DeepEqual(t, data, ts)

	// mimic 2nd forwarder reading from server
	mockedReadValue2[util.TimeSyncUUID] = bytes.NewBuffer(ts)
	readChar2 := readChars2[0]
	data, err = readChar2.HandleRead(testAddr, context.Background())
	assert.NilError(t, err)
	assert.DeepEqual(t, data, ts)
}
