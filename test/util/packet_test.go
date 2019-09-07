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

func TestGetPacketsFromData(t *testing.T) {
	data := getRandBytes(t)
	packets := GetPacketsFromData(data)
	assert.Equal(t, len(packets), numPackets)
	for index, packet := range packets {
		assert.Assert(t, len(packet.Guid) > 0)
		assert.Equal(t, packet.Index, index)
		assert.Equal(t, packet.Total, numPackets)
		assert.Assert(t, len(packet.Checksum) > 0)
	}
}

func TestAddPacketAndGetPacketStreams(t *testing.T) {
	packet := getSamplePacket()
	packetData, err := packet.Data()
	assert.NilError(t, err)
	pa := NewPacketAggregator()
	err = pa.AddPacketFromPacketBytes(packetData)
	assert.NilError(t, err)
	actual := pa.GetPacketStreams()
	assert.DeepEqual(t, actual, []string{dummyGuid})
}

func TestGetDataFromPackets(t *testing.T) {
	expected := getRandBytes(t)
	packets := GetPacketsFromData(expected)
	pa := NewPacketAggregator()
	guid := packets[0].Guid
	for _, packet := range packets {
		packetData, err := packet.Data()
		assert.NilError(t, err)
		err = pa.AddPacketFromPacketBytes(packetData)
		assert.NilError(t, err)
	}
	actual, err := pa.GetDataFromPackets(guid)
	assert.NilError(t, err)
	assert.DeepEqual(t, actual, expected)
}
