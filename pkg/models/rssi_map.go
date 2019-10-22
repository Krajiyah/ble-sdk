package models

import (
	"bytes"
	"encoding/gob"
)

// RssiMap is type alias for rssi map (routing tables)
type RssiMap map[string]map[string]int

// Data will return serialized form of struct as bytes
func (rm *RssiMap) Data() ([]byte, error) {
	return encode(rm)
}

// Merge will write input rssiMap to this rssiMap
func (rm *RssiMap) Merge(o *RssiMap) {
	actualRM := *rm
	actualO := *o
	for addr := range actualO {
		for nestedAddr := range actualO[addr] {
			actualRM[addr][nestedAddr] = actualO[addr][nestedAddr]
		}
	}
}

// GetRssiMapFromBytes constructs client state request from characteristic write payload
func GetRssiMapFromBytes(data []byte) (*RssiMap, error) {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	var ret RssiMap
	err := dec.Decode(&ret)
	if err != nil {
		return nil, err
	}
	return &ret, nil
}
