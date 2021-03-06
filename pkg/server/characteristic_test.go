package server

import (
	"bytes"
	"context"
	"testing"

	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
	"gotest.tools/assert"
)

const (
	dummyAddr = "11:22:33:44:55:66"
)

type mockConn struct {
	ctx context.Context
}

func (c *mockConn) Context() context.Context          { return c.ctx }
func (c *mockConn) SetContext(ctx context.Context)    { c.ctx = ctx }
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

type mockRspWriter struct {
	buff *bytes.Buffer
}

func (rw *mockRspWriter) ReadAll() []byte {
	return rw.buff.Bytes()
}

func (rw *mockRspWriter) Write(b []byte) (int, error) {
	return rw.buff.Write(b)
}
func (rw *mockRspWriter) Status() ble.ATTError          { return ble.ErrSuccess }
func (rw *mockRspWriter) SetStatus(status ble.ATTError) {}
func (rw *mockRspWriter) Len() int                      { return rw.buff.Len() }
func (rw *mockRspWriter) Cap() int                      { return rw.buff.Cap() }

type testBlankListener struct{}

func (l testBlankListener) OnClientStateMapChanged(c *ConnectionGraph, r *RssiMap, m map[string]BLEClientState) {
}
func (l testBlankListener) OnClientLog(r ClientLogRequest) {}
func (l testBlankListener) OnInternalError(err error)      {}

func getDummyServer() *BLEServer {
	secret := "passwd123"
	return &BLEServer{"SomeName", "someAddr", secret, NewConnectionGraph(), NewRssiMap(), map[string]BLEClientState{}, util.NewPacketBuffer(secret), testBlankListener{}}
}

func getMockReq(data []byte) ble.Request {
	return ble.NewRequest(&mockConn{ctx: context.Background()}, data, 0)
}

func getMockRsp(data []byte) *mockRspWriter {
	return &mockRspWriter{buff: bytes.NewBuffer(data)}
}

func TestWriteHandler(t *testing.T) {
	server := getDummyServer()
	expected := []byte("Hello World!")
	packets, _, err := util.EncodeDataAsPackets(expected, server.secret)
	assert.NilError(t, err)
	callCount := 0
	handler := generateWriteHandler(server, util.MainServiceUUID, func(addr string, actual []byte, err error) {
		assert.Equal(t, addr, dummyAddr)
		assert.NilError(t, err)
		assert.DeepEqual(t, expected, actual)
		callCount++
	})

	// test write handler behavior
	for _, packet := range packets {
		handler(getMockReq(packet), getMockRsp([]byte{}))
	}

	assert.Equal(t, callCount, 1)
}

func TestReadHandler(t *testing.T) {
	server := getDummyServer()
	expected := []byte("Hello World!")
	handler := generateReadHandler(server, util.MainServiceUUID, func(addr string, c context.Context) ([]byte, error) {
		return expected, nil
	})

	req := getMockReq([]byte{})
	rsp := getMockRsp([]byte{})

	// test read handler behavior
	handler(req, rsp)

	// mock client read and decrypt
	packet := rsp.ReadAll()
	buffer := util.NewPacketBuffer(server.secret)
	actual, err := buffer.Set(packet)
	assert.NilError(t, err)
	assert.DeepEqual(t, actual, expected)
}
