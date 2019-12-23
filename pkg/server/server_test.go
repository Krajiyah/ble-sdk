package server

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"gotest.tools/assert"
)

var errs []error
var state map[string]BLEClientState
var logs []ClientLogRequest

type testBLEServerStatusListener struct{}

func (l testBLEServerStatusListener) OnServerStatusChanged(s BLEServerStatus, err error) {
	errs = append(errs, err)
}
func (l testBLEServerStatusListener) OnClientStateMapChanged(m map[string]BLEClientState) { state = m }
func (l testBLEServerStatusListener) OnClientLog(r ClientLogRequest)                      { logs = append(logs, r) }
func (l testBLEServerStatusListener) OnReadOrWriteError(err error)                        { errs = append(errs, err) }

func assertSimilar(t *testing.T, x int64, y int64) {
	diff := x - y
	if diff < 0 {
		diff *= -1
	}
	assert.Assert(t, diff < 50)
}

func getTestServer() *BLEServer {
	return newBLEServer("SomeName", "passwd123", testBLEServerStatusListener{})
}

func dummyReadChar() *BLEReadCharacteristic {
	return &BLEReadCharacteristic{"10010000-0001-1000-8000-00805F9B34FB", func(string, context.Context) ([]byte, error) { return nil, nil }, func() {}}
}

func dummyWriteChar() *BLEWriteCharacteristic {
	return &BLEWriteCharacteristic{"20010000-0001-1000-8000-00805F9B34FB", func(string, []byte, error) {}, func() {}}
}

func beforeEach() {
	errs = []error{}
	state = map[string]BLEClientState{}
	logs = []ClientLogRequest{}
}

func TestStatusSetters(t *testing.T) {
	beforeEach()
	server := getTestServer()
	expected := errors.New("test error")
	server.setStatus(Crashed, expected)
	assert.Equal(t, server.status, Crashed)
	assert.Equal(t, len(errs), 1)
	assert.DeepEqual(t, errs[0].Error(), expected.Error())
	addr := "someaddr"
	expectedRM := map[string]map[string]int{"a": map[string]int{"b": -90}}
	s := BLEClientState{Connected, expectedRM}
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
	m := NewRssiMap()
	m.Set(addr, "some other addr", -80)
	req := ClientStateRequest{m.GetAll()}
	b, err := req.Data()
	assert.NilError(t, err)
	go char.DoInBackground()
	char.HandleWrite(addr, b, nil)
	assert.Equal(t, server.clientStateMap[addr].Status, Connected)
	stateMap := server.clientStateMap[addr].RssiMap
	assert.DeepEqual(t, stateMap, m.GetAll())
	time.Sleep(PollingInterval * 3)
	stateMap = server.clientStateMap[addr].RssiMap
	assert.Equal(t, server.clientStateMap[addr].Status, Disconnected)
	assert.DeepEqual(t, stateMap, map[string]map[string]int{})
}
