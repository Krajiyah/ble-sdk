package models

import (
	"bytes"
	"encoding/gob"
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
	RssiMap map[string]map[string]int
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

func encode(x interface{}) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	enc := gob.NewEncoder(buf)
	err := enc.Encode(x)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Data will return serialized form of struct as bytes
func (c *ClientStateRequest) Data() ([]byte, error) {
	return encode(c)
}

// Data will return serialized form of struct as bytes
func (c *ClientLogRequest) Data() ([]byte, error) {
	return encode(c)
}

// Data will return serialized form of struct as bytes
func (f *ForwarderRequest) Data() ([]byte, error) {
	return encode(f)
}

func getDecoder(data []byte) *gob.Decoder {
	buf := bytes.NewBuffer(data)
	return gob.NewDecoder(buf)
}

// GetClientStateRequestFromBytes constructs client state request from characteristic write payload
func GetClientStateRequestFromBytes(data []byte) (*ClientStateRequest, error) {
	dec := getDecoder(data)
	var ret ClientStateRequest
	err := dec.Decode(&ret)
	return &ret, err
}

// GetClientLogRequestFromBytes constructs client log request from characteristic write payload
func GetClientLogRequestFromBytes(data []byte) (*ClientLogRequest, error) {
	dec := getDecoder(data)
	var ret ClientLogRequest
	err := dec.Decode(&ret)
	return &ret, err
}

// GetForwarderRequestFromBytes constructs forwarder request from characteristic write payload
func GetForwarderRequestFromBytes(data []byte) (*ForwarderRequest, error) {
	dec := getDecoder(data)
	var ret ForwarderRequest
	err := dec.Decode(&ret)
	return &ret, err
}
