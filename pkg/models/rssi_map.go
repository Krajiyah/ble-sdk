package models

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"strings"
	"sync"
)

type RssiMapData map[string]map[string]int

// RssiMap is type alias for rssi map (routing tables)
type RssiMap struct {
	data  RssiMapData
	mutex sync.RWMutex
}

// NewRssiMap will return newly init struct
func NewRssiMap() *RssiMap {
	return &RssiMap{data: RssiMapData{}, mutex: sync.RWMutex{}}
}

// NewRssiMapFromRaw will return newly init struct
func NewRssiMapFromRaw(raw RssiMapData) *RssiMap {
	rm := NewRssiMap()
	for src, f := range raw {
		for dst, rssi := range f {
			rm.Set(src, dst, rssi)
		}
	}
	return rm
}

// Data will return serialized form of struct as bytes
func (rm *RssiMap) Data() ([]byte, error) {
	return encode(rm.GetAll())
}

// Set will update the map
func (rm *RssiMap) Set(src, dst string, new int) {
	src = strings.ToUpper(src)
	dst = strings.ToUpper(dst)
	rm.mutex.Lock()
	if _, ok := rm.data[src]; !ok {
		rm.data[src] = map[string]int{}
	}
	rm.data[src][dst] = new
	rm.mutex.Unlock()
}

// GetAll will get all data from map
func (rm *RssiMap) GetAll() RssiMapData {
	return rm.data
}

// Get will get from map
func (rm *RssiMap) Get(src, dst string) (int, bool) {
	src = strings.ToUpper(src)
	dst = strings.ToUpper(dst)
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

// String returns json string of data
func (rm *RssiMap) String() string {
	b, _ := json.Marshal(rm.GetAll())
	return string(b)
}

// GetRssiMapFromBytes constructs client state request from characteristic write payload
func GetRssiMapFromBytes(data []byte) (*RssiMap, error) {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	var m RssiMapData
	err := dec.Decode(&m)
	if err != nil {
		return nil, err
	}
	return NewRssiMapFromRaw(m), nil
}
