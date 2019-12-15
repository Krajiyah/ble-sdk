package models

import (
	"bytes"
	"encoding/gob"
	"sync"
)

// RssiMap is type alias for rssi map (routing tables)
type RssiMap struct {
	data  map[string]map[string]int
	mutex sync.RWMutex
}

// NewRssiMap will return newly init struct
func NewRssiMap() RssiMap {
	return RssiMap{data: map[string]map[string]int{}, mutex: sync.RWMutex{}}
}

// Data will return serialized form of struct as bytes
func (rm *RssiMap) Data() ([]byte, error) {
	return encode(rm.GetAll())
}

// Set will update the map
func (rm *RssiMap) Set(src, dst string, new int) {
	rm.mutex.Lock()
	if _, ok := rm.data[src]; !ok {
		rm.data[src] = map[string]int{}
	}
	rm.data[src][dst] = new
	rm.mutex.Unlock()
}

// GetAll will get all data from map
func (rm *RssiMap) GetAll() map[string]map[string]int {
	return rm.data
}

// Get will get from map
func (rm *RssiMap) Get(src, dst string) (int, bool) {
	rm.mutex.RLock()
	if tmp, ok := rm.data[src]; ok {
		ret, oke := tmp[dst]
		return ret, oke
	}
	rm.mutex.RUnlock()
	return 0, false
}

// Merge will write input rssiMap to this rssiMap
func (rm *RssiMap) Merge(o *RssiMap) {
	for addr := range o.data {
		for nestedAddr := range o.data[addr] {
			val, _ := o.Get(addr, nestedAddr)
			rm.Set(addr, nestedAddr, val)
		}
	}
}

// GetRssiMapFromBytes constructs client state request from characteristic write payload
func GetRssiMapFromBytes(data []byte) (*RssiMap, error) {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	var m map[string]map[string]int
	err := dec.Decode(&m)
	if err != nil {
		return nil, err
	}
	rm := NewRssiMap()
	rm.data = m
	return &rm, nil
}
