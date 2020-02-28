package util

import (
	"testing"

	"gotest.tools/assert"
)

const (
	largeDataSize = 5 * 1000 * 1000 // 5 MB
	smallDataSize = 200             // 200 B
	secret        = "passwd123"
)

func TestBytesToNum(t *testing.T) {
	expected := uint32(123)
	x := numToBytes(expected)
	actual := bytesToNum(x)
	assert.Equal(t, expected, actual)
}

func testEncodeAndBuff(size int) func(*testing.T) {
	return func(t *testing.T) {
		expected := getRandBytes(size)
		packets, err := EncodeDataAsPackets(expected, secret)
		assert.NilError(t, err)
		buff := NewPacketBuffer(secret)
		var actual []byte
		for _, packet := range packets {
			actual, err = buff.Set(packet)
			assert.NilError(t, err)
		}
		assert.DeepEqual(t, expected, actual)
	}
}

func TestEncodeAndBuff(t *testing.T) {
	t.Run("Small", testEncodeAndBuff(smallDataSize))
	t.Run("Large", testEncodeAndBuff(largeDataSize))
}
