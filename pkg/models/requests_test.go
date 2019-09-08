package models

import (
	"testing"

	"gotest.tools/assert"
)

func TestClientStateRequest(t *testing.T) {
	expected := ClientStateRequest{map[string]int{"someAddr": -90}}
	enc, err := expected.Data()
	assert.NilError(t, err)
	actual, err := GetClientStateRequestFromBytes(enc)
	assert.NilError(t, err)
	assert.DeepEqual(t, *actual, expected)
}

func TestClientLogRequest(t *testing.T) {
	expected := ClientLogRequest{Info, "Some message"}
	enc, err := expected.Data()
	assert.NilError(t, err)
	actual, err := GetClientLogRequestFromBytes(enc)
	assert.NilError(t, err)
	assert.DeepEqual(t, *actual, expected)
}
