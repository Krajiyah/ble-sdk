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

func getTestServer() *BLEServer {
	l := BLEServerStatusListener{
		func(s BLEServerStatus, err error) { errs = append(errs, err) },
		func(m map[string]BLEClientState) { state = m },
		func(r ClientLogRequest) { logs = append(logs, r) },
		func(err error) { errs = append(errs, err) },
	}
	return &BLEServer{"SomeName", "passwd123", Running, map[string]BLEClientState{}, util.NewPacketAggregator(), l}
}

func dummyReadChar() *BLEReadCharacteristic {
	return &BLEReadCharacteristic{"10010000-0001-1000-8000-00805F9B34FB", func(context.Context) ([]byte, error) { return nil, nil }, func() {}}
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
	s := BLEClientState{Disconnected, map[string]int{}}
	server.setClientState(addr, s)
	expectedState := map[string]BLEClientState{}
	expectedState[addr] = s
	assert.DeepEqual(t, server.clientStateMap, expectedState)
	assert.DeepEqual(t, server.clientStateMap, state)
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
	b, err := char.HandleRead(context.Background())
	assert.NilError(t, err)
	ts, err := strconv.Atoi(string(b))
	assert.Equal(t, int64(ts), util.UnixTS())
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
	m := map[string]int{"some other addr": -80}
	req := ClientStateRequest{m}
	b, err := req.Data()
	assert.NilError(t, err)
	go char.DoInBackground()
	char.HandleWrite(addr, b, nil)
	assert.DeepEqual(t, server.clientStateMap, map[string]BLEClientState{addr: BLEClientState{Connected, m}})
	time.Sleep(PollingInterval * 3)
	assert.DeepEqual(t, server.clientStateMap, map[string]BLEClientState{addr: BLEClientState{Disconnected, map[string]int{}}})
}