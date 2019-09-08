package server

import (
	"bytes"
	"context"
	"math/rand"
	"testing"

	"github.com/Krajiyah/ble-sdk/pkg/client"
	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/currantlabs/ble"
	"gotest.tools/assert"
)

const (
	dummyAddr = "11:22:33:44:55:66"
)

var (
	testReadHandlerBuffer       *bytes.Buffer
	testReadHandlerPacketBuffer *bytes.Buffer
)

type emptyListener struct{}

func (l *emptyListener) onServerStatusChanged(s BLEServerStatus, err error)         {}
func (l *emptyListener) onClientStateMapChanged(m map[string]client.BLEClientState) {}
func (l *emptyListener) onClientLog(r *models.ClientLogRequest)                     {}

type mockConn struct{}

func (c *mockConn) Context() context.Context          { return context.Background() }
func (c *mockConn) SetContext(ctx context.Context)    {}
func (c *mockConn) LocalAddr() ble.Addr               { return ble.NewAddr(dummyAddr) }
func (c *mockConn) RemoteAddr() ble.Addr              { return ble.NewAddr(dummyAddr) }
func (c *mockConn) RxMTU() int                        { return util.MTU }
func (c *mockConn) SetRxMTU(mtu int)                  {}
func (c *mockConn) TxMTU() int                        { return util.MTU }
func (c *mockConn) SetTxMTU(mtu int)                  {}
func (c *mockConn) Disconnected() <-chan struct{}     { return make(chan struct{}) }
func (c *mockConn) Read(p []byte) (n int, err error)  { return 0, nil }
func (c *mockConn) Write(p []byte) (n int, err error) { return 0, nil }
func (c *mockConn) Close() error                      { return nil }

type mockRspWriter struct{}

func (rw *mockRspWriter) Write(b []byte) (int, error) {
	testReadHandlerPacketBuffer = bytes.NewBuffer(b)
	return testReadHandlerBuffer.Write(b)
}
func (rw *mockRspWriter) Status() ble.ATTError          { return ble.ErrSuccess }
func (rw *mockRspWriter) SetStatus(status ble.ATTError) {}
func (rw *mockRspWriter) Len() int                      { return testReadHandlerBuffer.Len() }
func (rw *mockRspWriter) Cap() int                      { return testReadHandlerBuffer.Cap() }

func getRandBytes(t *testing.T) []byte {
	b := make([]byte, util.MTU*3)
	_, err := rand.Read(b)
	assert.NilError(t, err)
	return b
}

func getDummyServer() *BLEServer {
	return &BLEServer{"SomeName", "passwd123", Running, map[string]client.BLEClientState{}, util.NewPacketAggregator(), &emptyListener{}}
}

func getMockReq(data []byte) ble.Request {
	return ble.NewRequest(&mockConn{}, data, 0)
}

func getMockRsp(data []byte) ble.ResponseWriter {
	return &mockRspWriter{}
}

func TestWriteHandler(t *testing.T) {
	server := getDummyServer()
	expected := getRandBytes(t)
	encData, err := util.Encrypt(expected, server.secret)
	assert.NilError(t, err)
	pa := util.NewPacketAggregator()
	guid, err := pa.AddData(encData)
	assert.NilError(t, err)
	shouldOnWriteNow := false
	wasCalled := false
	handler := generateWriteHandler(server, MainServiceUUID, func(addr string, actual []byte, err error) {
		assert.Assert(t, shouldOnWriteNow)
		assert.Equal(t, addr, dummyAddr)
		assert.NilError(t, err)
		assert.DeepEqual(t, actual, expected)
		wasCalled = true
	})

	for !wasCalled {

		// mock client writing
		packetData, isLastPacket, err := pa.PopPacketDataFromStream(guid)
		assert.NilError(t, err)
		shouldOnWriteNow = isLastPacket
		req := getMockReq(packetData)
		rsp := getMockRsp([]byte{})

		// test write handler behavior
		handler(req, rsp)
	}
}

func TestReadHandler(t *testing.T) {
	testReadHandlerBuffer = bytes.NewBuffer([]byte{})
	server := getDummyServer()
	expected := getRandBytes(t)
	pa := util.NewPacketAggregator()
	handler := generateReadHandler(server, MainServiceUUID, func(c context.Context) ([]byte, error) {
		return expected, nil
	})
	guid := ""

	for !pa.HasDataFromPacketStream(guid) {
		// test read handler behavior
		req := getMockReq([]byte{})
		rsp := getMockRsp([]byte{})
		handler(req, rsp)

		// mock client read
		var err error
		guid, err = pa.AddPacketFromPacketBytes(testReadHandlerPacketBuffer.Bytes())
		// TODO: fix bug guid changes every new packet comes in
		assert.NilError(t, err)
	}

	encData, err := pa.PopAllDataFromPackets(guid)
	assert.NilError(t, err)
	actual, err := util.Decrypt(encData, server.secret)
	assert.NilError(t, err)
	assert.DeepEqual(t, actual, expected)
}
