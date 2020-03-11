package server

import (
	"context"
	"strconv"
	"testing"
	"time"

	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"gotest.tools/assert"
)

var state map[string]BLEClientState
var logs []ClientLogRequest

type testBLEServerStatusListener struct{}

func (l testBLEServerStatusListener) OnClientStateMapChanged(c *ConnectionGraph, r *RssiMap, m map[string]BLEClientState) {
	state = m
}
func (l testBLEServerStatusListener) OnClientLog(r ClientLogRequest) { logs = append(logs, r) }
func (l testBLEServerStatusListener) OnInternalError(err error)      {}

func assertSimilar(t *testing.T, x int64, y int64) {
	diff := x - y
	if diff < 0 {
		diff *= -1
	}
	assert.Assert(t, diff < 50)
}

func getTestServer() *BLEServer {
	return newBLEServer("SomeName", "someAddr", "passwd123", testBLEServerStatusListener{})
}

func dummyReadChar() *BLEReadCharacteristic {
	return &BLEReadCharacteristic{"10010000-0001-1000-8000-00805F9B34FB", func(string, context.Context) ([]byte, error) { return nil, nil }, func() {}}
}

func dummyWriteChar() *BLEWriteCharacteristic {
	return &BLEWriteCharacteristic{"20010000-0001-1000-8000-00805F9B34FB", func(string, []byte, error) {}, func() {}}
}

func beforeEach() {
	state = map[string]BLEClientState{}
	logs = []ClientLogRequest{}
}

func TestStatusSetters(t *testing.T) {
	beforeEach()
	server := getTestServer()
	addr := "someaddr"
	expectedRM := map[string]map[string]int{"A": map[string]int{"B": -90}}
	s := BLEClientState{Name: "someName", Status: Connected, RssiMap: expectedRM, ConnectedAddr: addr}
	server.setClientState(addr, s)
	expectedState := map[string]BLEClientState{}
	expectedState[addr] = s
	assert.DeepEqual(t, server.clientStateMap, expectedState)
	assert.DeepEqual(t, server.clientStateMap, state)
	rm := server.GetRssiMap()
	assert.DeepEqual(t, rm.GetAll(), expectedRM)
}

func TestGetService(t *testing.T) {
	beforeEach()
	server := getTestServer()
	service := getService(server, []*BLEReadCharacteristic{dummyReadChar()}, []*BLEWriteCharacteristic{dummyWriteChar()})
	assert.Equal(t, len(service.Characteristics), 5)
}

func TestTimeSyncChar(t *testing.T) {
	beforeEach()
	server := getTestServer()
	char := newTimeSyncChar(server)
	b, err := char.HandleRead("some addr", context.Background())
	assert.NilError(t, err)
	ts, err := strconv.Atoi(string(b))
	assertSimilar(t, int64(ts), util.UnixTS())
}

func TestClientLogChar(t *testing.T) {
	beforeEach()
	server := getTestServer()
	char := newClientLogChar(server)
	req := ClientLogRequest{"some addr", Info, "some message"}
	b, err := req.Data()
	assert.NilError(t, err)
	char.HandleWrite("some addr", b, nil)
	assert.DeepEqual(t, logs, []ClientLogRequest{req})
}

func TestClientStatusChar(t *testing.T) {
	beforeEach()
	server := getTestServer()
	char := newClientStatusChar(server)
	addr := "some addr"
	other := "some other addr"
	m := NewRssiMap()
	m.Set(addr, other, -80)
	req := ClientStateRequest{Addr: addr, ConnectedAddr: other, RssiMap: m.GetAll()}
	b, err := req.Data()
	assert.NilError(t, err)
	go char.DoInBackground()
	char.HandleWrite(addr, b, nil)
	assert.Equal(t, server.clientStateMap[addr].Status, Connected)
	stateMap := server.clientStateMap[addr].RssiMap
	assert.DeepEqual(t, stateMap, m.GetAll())
	assert.DeepEqual(t, server.GetRssiMap().GetAll(), m.GetAll())
	assert.DeepEqual(t, server.GetConnectionGraph().GetAll(), NewConnectionGraphFromRaw(map[string]string{addr: server.addr}).GetAll())
	time.Sleep(PollingInterval * 3)
	stateMap = server.clientStateMap[addr].RssiMap
	assert.Equal(t, server.clientStateMap[addr].Status, Disconnected)
	assert.DeepEqual(t, stateMap, map[string]map[string]int{})
}
