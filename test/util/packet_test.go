package test

import (
	"math/rand"
	"testing"

	. "github.com/Krajiyah/ble-sdk/pkg/util"
	"gotest.tools/assert"
)

const (
	numPackets = 3
	dummyGuid  = "someUniqueValue"
)

func getRandBytes(t *testing.T) []byte {
	b := make([]byte, MTU*numPackets)
	_, err := rand.Read(b)
	assert.NilError(t, err)
	return b
}

func getSamplePacket() BLEPacket {
	packet := BLEPacket{}
	packet.RawData = []byte("Hello World!")
	packet.Guid = dummyGuid
	packet.Index = 0
	packet.Total = 1
	packet.Checksum = GetChecksum(packet.RawData)
	return packet
}

func TestAddPacketAndGetPacketStreams(t *testing.T) {
	packet := getSamplePacket()
	packetData, err := packet.Data()
	assert.NilError(t, err)
	pa := NewPacketAggregator()
	g, err := pa.AddPacketFromPacketBytes(packetData)
	assert.NilError(t, err)
	assert.Equal(t, packet.Guid, g)
	ok := pa.HasDataFromPacketStream(g)
	assert.Assert(t, ok)
}

func TestGetDataFromPackets(t *testing.T) {
	expected := getRandBytes(t)
	pa := NewPacketAggregator()
	guid, err := pa.AddData(expected)
	assert.NilError(t, err)
	actual, err := pa.PopAllDataFromPackets(guid)
	assert.NilError(t, err)
	assert.DeepEqual(t, actual, expected)
}
