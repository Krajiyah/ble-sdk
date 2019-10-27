package client

import (
	"strconv"
	"testing"

	"github.com/Krajiyah/ble-sdk/internal"
	. "github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/server"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/currantlabs/ble"
	"gotest.tools/assert"
)

const (
	testAddr       = "11:22:33:44:55:66"
	testServerAddr = "22:22:33:44:55:66"
	testSecret     = "test123"
)

var (
	attempts       = 0
	rssi           = 0
	isDisconnected = false
)

func setCharacteristic(client *BLEClient, uuid string) {
	u, err := ble.Parse(uuid)
	if err != nil {
		panic("could not mock read characteristic")
	}
	client.characteristics[uuid] = ble.NewCharacteristic(u)
}

func dummyOnConnected(a int, r int) {
	attempts = a
	rssi = r
	isDisconnected = false
}

func dummyOnDisconnected() {
	isDisconnected = true
}

func getDummyClient() *BLEClient {
	client := &BLEClient{
		testAddr, testSecret, Disconnected, 0, nil, testServerAddr, map[string]int{}, util.MakeINFContext(), nil,
		map[string]*ble.Characteristic{}, util.NewPacketAggregator(), dummyOnConnected, dummyOnDisconnected,
	}
	dummyCoreClient := internal.NewDummyCoreClient(testAddr)
	var c ble.Client
	c = dummyCoreClient
	client.cln = &c
	return client
}

func TestUnixTS(t *testing.T) {
	client := getDummyClient()

	// mock server read
	pa := util.NewPacketAggregator()
	expected := util.UnixTS()
	encData, err := util.Encrypt([]byte(strconv.Itoa(int(expected))), testSecret)
	assert.NilError(t, err)
	guid, err := pa.AddData(encData)
	assert.NilError(t, err)
	var isLastPacket bool
	internal.MockReadCharData, isLastPacket, err = pa.PopPacketDataFromStream(guid)
	assert.NilError(t, err)
	assert.Assert(t, isLastPacket)

	// test client read
	setCharacteristic(client, server.TimeSyncUUID)
	ts, err := client.getUnixTS()
	assert.NilError(t, err)
	assert.Equal(t, ts, expected)
}

func TestLog(t *testing.T) {
	client := getDummyClient()

	// test client write
	setCharacteristic(client, server.ClientLogUUID)
	expected := ClientLogRequest{"SomeAddress", Info, "Some Message"}
	err := client.Log(expected)
	assert.NilError(t, err)

	// mock write to server
	data, err := expected.Data()
	assert.NilError(t, err)
	encData, err := util.Encrypt(data, client.secret)
	assert.NilError(t, err)
	pa := util.NewPacketAggregator()
	guid1, err := pa.AddPacketFromPacketBytes(internal.MockWriteCharData[0])
	assert.NilError(t, err)
	guid2, err := pa.AddPacketFromPacketBytes(internal.MockWriteCharData[1])
	assert.NilError(t, err)
	guid3, err := pa.AddPacketFromPacketBytes(internal.MockWriteCharData[2])
	assert.NilError(t, err)
	assert.Equal(t, guid1, guid2)
	assert.Equal(t, guid1, guid3)
	ok := pa.HasDataFromPacketStream(guid1)
	assert.Assert(t, ok)
	encData, err = pa.PopAllDataFromPackets(guid1)
	assert.NilError(t, err)
	data, err = util.Decrypt(encData, client.secret)
	assert.NilError(t, err)
	actual, err := GetClientLogRequestFromBytes(data)
	assert.NilError(t, err)
	assert.DeepEqual(t, *actual, expected)
}
