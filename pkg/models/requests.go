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

// ClientStateRequest is the payload for the equivalent write characteristic
type ClientStateRequest struct {
	RssiMap map[string]int
}

// ClientLogRequest is the payload for the equivalent write characteristic
type ClientLogRequest struct {
	Level   LogLevel
	Message string
}

// GetClientStateRequestFromBytes constructs client state request from characteristic write payload
func GetClientStateRequestFromBytes(data []byte) (*ClientStateRequest, error) {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	var ret ClientStateRequest
	err := dec.Decode(&ret)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

// GetClientLogRequestFromBytes constructs client log request from characteristic write payload
func GetClientLogRequestFromBytes(data []byte) (*ClientLogRequest, error) {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	var ret ClientLogRequest
	err := dec.Decode(&ret)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}
