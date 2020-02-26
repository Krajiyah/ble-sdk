package util

import (
	"testing"

	"gotest.tools/assert"
)

type TestObj struct {
	X int
	Y string
	Z map[string]int
}

func TestCompressDecompress(t *testing.T) {
	expected := "Hello World!"
	data, err := compress([]byte(expected))
	assert.NilError(t, err)
	data2, err := decompress(data)
	assert.NilError(t, err)
	assert.Equal(t, expected, string(data2))
}

func TestEncodeDecode(t *testing.T) {
	expected := TestObj{X: 1, Y: "hello", Z: map[string]int{"world": 2}}
	data, err := Encode(expected)
	assert.NilError(t, err)
	var actual TestObj
	err = Decode(data, &actual)
	assert.NilError(t, err)
	assert.DeepEqual(t, expected, actual)
}
