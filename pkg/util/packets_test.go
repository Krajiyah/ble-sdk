package util

import (
	"testing"

	"gotest.tools/assert"
)

const (
	largeDataSize = 5 * 1000 * 1000 // 5 MB
	smallDataSize = 10              // 10 B
	secret        = "passwd123"
)

func TestEncodeDecodePacketsLarge(t *testing.T) {
	expected := getRandBytes(largeDataSize)
	packets, err := EncodeDataAsPackets(expected, secret)
	assert.NilError(t, err)
	actual, err := DecodePacketsToData(packets, secret)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, actual)
}

func TestEncodeDecodePacketsSmall(t *testing.T) {
	expected := getRandBytes(smallDataSize)
	packets, err := EncodeDataAsPackets(expected, secret)
	assert.NilError(t, err)
	actual, err := DecodePacketsToData(packets, secret)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, actual)
}
