package models

import (
	"testing"

	"gotest.tools/assert"
)

func TestClientStateRequest(t *testing.T) {
	rm := NewRssiMap()
	rm.Set("someOtherAddr", "someAddr", -90)
	expected := ClientStateRequest{rm.GetAll()}
	enc, err := expected.Data()
	assert.NilError(t, err)
	actual, err := GetClientStateRequestFromBytes(enc)
	assert.NilError(t, err)
	assert.DeepEqual(t, *actual, expected)
}

func TestClientLogRequest(t *testing.T) {
	expected := ClientLogRequest{"someaddr", Info, "Some message"}
	enc, err := expected.Data()
	assert.NilError(t, err)
	actual, err := GetClientLogRequestFromBytes(enc)
	assert.NilError(t, err)
	assert.DeepEqual(t, *actual, expected)
}
