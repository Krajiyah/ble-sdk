package util

import (
	"testing"

	"gotest.tools/assert"
)

const (
	largeDataSize = 5 * 1000 * 1000 // 5 MB
	smallDataSize = 50              // 50 B
	secret        = "passwd123"
)

func TestBytesToNum(t *testing.T) {
	expected := uint32(123)
	x := numToBytes(expected)
	actual := bytesToNum(x)
	assert.Equal(t, expected, actual)
}

func testBuffer(size int) func(*testing.T) {
	return func(t *testing.T) {
		expected := getRandBytes(size)
		packets, guid, err := EncodeDataAsPackets(expected, secret)
		assert.NilError(t, err)
		serverBuff := NewPacketBuffer(secret)
		clientBuff := NewPacketBuffer(secret)
		err = serverBuff.SetAll(packets)
		assert.NilError(t, err)
		last := false
		var actual []byte
		i := 0
		for i < len(packets) {
			var packet []byte
			packet, last, err = serverBuff.Pop(guid)
			assert.NilError(t, err)
			assert.Check(t, last == (i == len(packets)-1))
			assert.Equal(t, len(packet), len(packets[i]))
			actual, err = clientBuff.Set(packet)
			assert.NilError(t, err)
			i++
		}
		assert.DeepEqual(t, expected, actual)
	}
}

func TestBufferSmall(t *testing.T) {
	testBuffer(smallDataSize)(t)
}

func TestBufferLarge(t *testing.T) {
	testBuffer(largeDataSize)(t)
}
