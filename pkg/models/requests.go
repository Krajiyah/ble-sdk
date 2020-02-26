package models

import (
	"github.com/Krajiyah/ble-sdk/pkg/util"
)

// LogLevel is the enum for client to server logging levels
type LogLevel int

const (
	Info LogLevel = iota
	Debug
	Warning
	Error
)

func (l LogLevel) String() string {
	return []string{"Info", "Debug", "Warning", "Error"}[l]
}

// ClientStateRequest is the payload for the equivalent write characteristic
type ClientStateRequest struct {
	Name          string
	Addr          string
	ConnectedAddr string
	RssiMap       map[string]map[string]int
}

// ClientLogRequest is the payload for the equivalent write characteristic
type ClientLogRequest struct {
	Addr    string
	Level   LogLevel
	Message string
}

// ForwarderRequest is the wrapped payload for forwarding data
type ForwarderRequest struct {
	CharUUID string
	Payload  []byte
	IsRead   bool
	IsWrite  bool
}

// Data will return serialized form of struct as bytes
func (c *ClientStateRequest) Data() ([]byte, error) {
	return util.Encode(c)
}

// Data will return serialized form of struct as bytes
func (c *ClientLogRequest) Data() ([]byte, error) {
	return util.Encode(c)
}

// Data will return serialized form of struct as bytes
func (f *ForwarderRequest) Data() ([]byte, error) {
	return util.Encode(f)
}

// GetClientStateRequestFromBytes constructs client state request from characteristic write payload
func GetClientStateRequestFromBytes(data []byte) (*ClientStateRequest, error) {
	var ret ClientStateRequest
	err := util.Decode(data, &ret)
	return &ret, err
}

// GetClientLogRequestFromBytes constructs client log request from characteristic write payload
func GetClientLogRequestFromBytes(data []byte) (*ClientLogRequest, error) {
	var ret ClientLogRequest
	err := util.Decode(data, &ret)
	return &ret, err
}

// GetForwarderRequestFromBytes constructs forwarder request from characteristic write payload
func GetForwarderRequestFromBytes(data []byte) (*ForwarderRequest, error) {
	var ret ForwarderRequest
	err := util.Decode(data, &ret)
	return &ret, err
}
