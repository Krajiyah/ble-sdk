package forwarder

import (
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	. "github.com/Krajiyah/ble-sdk/internal"
	"github.com/Krajiyah/ble-sdk/pkg/client"
	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"

	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/currantlabs/ble"
	"gotest.tools/assert"
)

const (
	clientAddr     = "33:22:33:44:55:66"
	testServerName = "Some Name"
	testAddr       = "11:22:33:44:55:66"
	testAddr2      = "44:22:33:44:55:66"
	testSecret     = "passwd123"
	testServerAddr = "22:22:33:44:55:66"
)

type dummyListener struct{}

func (l dummyListener) OnServerStatusChanged(models.BLEServerStatus, error) {}
func (l dummyListener) OnConnectionError(error)                             {}
func (l dummyListener) OnReadOrWriteError(error)                            {}
func (l dummyListener) OnError(error)                                       {}
func (l dummyListener) OnClientConnected(string, int, int)                  {}
func (l dummyListener) OnClientDisconnected()                               {}

type dummyClient struct {
	addr              string
	dummyRssiMap      *RssiMap
	mockedReadValue   *bytes.Buffer
	mockedWriteBuffer *[]*bytes.Buffer
}

func (c dummyClient) RawScan(f func(ble.Advertisement)) error {
	for k, v := range c.dummyRssiMap.GetAll()[c.addr] {
		f(DummyAdv{DummyAddr{k}, v})
	}
	return nil
}

func (c dummyClient) ReadValue(uuid string) ([]byte, error) {
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

func getDummyForwarder(t *testing.T, addr string, rssiMap *RssiMap) *testStructs {
	mockedReadValue := bytes.NewBuffer([]byte{})
	mockedWriteBuffer := &[]*bytes.Buffer{}
	f := newBLEForwarder(addr, testServerAddr, dummyListener{})
	f.forwardingClient = dummyClient{addr, rssiMap, mockedReadValue, mockedWriteBuffer}
	f.forwardingServer = dummyServer{}
	return &testStructs{f, mockedReadValue, mockedWriteBuffer}
}

func TestSingleForwarder(t *testing.T) {
	expectedRssiMap := models.NewRssiMap()
	expectedRssiMap.Set(testAddr, testServerAddr, -90)
	expectedRssiMap.Set(testAddr, clientAddr, -10000)
	s := getDummyForwarder(t, testAddr, expectedRssiMap)
	forwarder := s.forwarder
	scan(forwarder, expectedRssiMap, testAddr)
	assert.DeepEqual(t, forwarder.rssiMap.GetAll(), expectedRssiMap.GetAll())
	assert.Equal(t, forwarder.toConnectAddr, forwarder.serverAddr)
	assert.Equal(t, forwarder.connectedAddr, forwarder.serverAddr)
}

func mockReadBuffer(t *testing.T, rssiMap *RssiMap, buffer *bytes.Buffer) {
	p, err := rssiMap.Data()
	assert.NilError(t, err)
	buffer.Write(p)
}

func scan(f *BLEForwarder, rssiMap *RssiMap, addr string) {
	for k, v := range rssiMap.GetAll()[addr] {
		f.onScanned(DummyAdv{DummyAddr{k}, v})
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
	scan(f1, expectedRssiMap, testAddr)
	scan(f2, expectedRssiMap, testAddr2)
	mockReadBuffer(t, f1.rssiMap, mockedReadValue2)
	mockReadBuffer(t, f2.rssiMap, mockedReadValue1)
	scan(f1, expectedRssiMap, testAddr)
	scan(f2, expectedRssiMap, testAddr2)
	assert.DeepEqual(t, f1.rssiMap.GetAll(), expectedRssiMap.GetAll())
	assert.DeepEqual(t, f2.rssiMap.GetAll(), expectedRssiMap.GetAll())
	assert.Equal(t, f1.toConnectAddr, testAddr2)
	assert.Equal(t, f1.connectedAddr, testAddr2)
	assert.Equal(t, f2.toConnectAddr, f2.serverAddr)
	assert.Equal(t, f2.connectedAddr, f2.serverAddr)
}

func TestRssiMapChar(t *testing.T) {
	rm := models.NewRssiMap()
	rm.Set(testAddr, testServerAddr, -90)
	rm.Set(testAddr, clientAddr, -10000)
	s := getDummyForwarder(t, testAddr, rm)
	s.forwarder.Run()
	time.Sleep(client.ScanInterval + (client.ScanInterval / 2))
	assert.DeepEqual(t, s.forwarder.rssiMap.GetAll(), rm.GetAll())
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
	mockReadBuffer(t, expectedRssiMap, mockedReadValue2)
	mockReadBuffer(t, expectedRssiMap, mockedReadValue1)
	f1.Run()
	f2.Run()
	time.Sleep(client.ScanInterval * 2)
	return s1, s2
}

func TestWriteChar(t *testing.T) {
	s1, s2 := prepare2ForwarderState(t)
	f1, _, mockedWriteBuffer1 := s1.forwarder, s1.mockedReadValue, s1.mockedWriteBuffer
	f2, _, mockedWriteBuffer2 := s2.forwarder, s2.mockedReadValue, s2.mockedWriteBuffer

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
	buffers1 := *mockedWriteBuffer1
	assert.Equal(t, len(buffers1), 1)
	bufferData1 := buffers1[0].Bytes()
	assert.DeepEqual(t, bufferData1, data)

	// mimic 2nd forwarder passing on data to server and unpacking forwarder request
	_, writeChars2 := getChars(f2)
	char2 := writeChars2[0]
	char2.HandleWrite(testAddr, data, nil)
	buffers2 := *mockedWriteBuffer2
	assert.Equal(t, len(buffers2), 1)
	bufferData2 := buffers2[0].Bytes()
	assert.DeepEqual(t, bufferData2, logData)
}

func TestStartEndReadChars(t *testing.T) {
	s1, s2 := prepare2ForwarderState(t)
	f1, mockedReadValue1, mockedWriteBuffer1 := s1.forwarder, s1.mockedReadValue, s1.mockedWriteBuffer
	f2, mockedReadValue2, mockedWriteBuffer2 := s2.forwarder, s2.mockedReadValue, s2.mockedWriteBuffer

	// mimic client write to forwarder (start read request)
	req := models.ForwarderRequest{util.TimeSyncUUID, nil, true, false}
	data, err := req.Data()
	assert.NilError(t, err)
	readChars1, writeChars1 := getChars(f1)
	writeChar1 := writeChars1[1]
	writeChar1.HandleWrite(clientAddr, data, nil)
	buffer1 := *mockedWriteBuffer1
	assert.Check(t, !f1.isConnectedToServer(), "F1 should not be connected to server")
	assert.Equal(t, len(buffer1), 1)
	assert.DeepEqual(t, buffer1[0].Bytes(), data)

	// mimic 2nd forwarder preparing for end read request
	readChars2, writeChars2 := getChars(f2)
	writeChar2 := writeChars2[1]
	writeChar2.HandleWrite(testAddr, data, nil)
	buffer2 := *mockedWriteBuffer2
	assert.Check(t, f2.isConnectedToServer(), "F2 should be connected to server")
	assert.Equal(t, len(buffer2), 0)

	// mimic client read from forwarder (end read request)
	ts := []byte(strconv.FormatInt(util.UnixTS(), 10))
	mockedReadValue1.Reset()
	mockedReadValue1.Write(ts)
	readChar1 := readChars1[0]
	data, err = readChar1.HandleRead(clientAddr, context.Background())
	assert.NilError(t, err)
	assert.DeepEqual(t, data, ts)

	// mimic 2nd forwarder reading from server
	mockedReadValue2.Reset()
	mockedReadValue2.Write(ts)
	readChar2 := readChars2[0]
	data, err = readChar2.HandleRead(testAddr, context.Background())
	assert.NilError(t, err)
	assert.DeepEqual(t, data, ts)
}
