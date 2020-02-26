package util

import (
	"testing"

	"gotest.tools/assert"
)

const (
	dataSize = 5 * 1000 * 1000 // 5 MB
)

func TestEncodeDecodePackets(t *testing.T) {
	expected := getRandBytes(dataSize)
	packets, err := EncodeDataAsPackets(expected)
	assert.NilError(t, err)
	actual, err := DecodePacketsToData(packets)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, actual)
}
