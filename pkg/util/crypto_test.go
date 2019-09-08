package util

import (
	"testing"

	"gotest.tools/assert"
)

func TestCrypto(t *testing.T) {
	expected := []byte("Hello World!")
	passwd := "Secret World...."
	encData, err := Encrypt(expected, passwd)
	assert.NilError(t, err)
	actual, err := Decrypt(encData, passwd)
	assert.NilError(t, err)
	assert.DeepEqual(t, actual, expected)
}
