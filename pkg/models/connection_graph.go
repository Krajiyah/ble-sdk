package models

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"strings"
	"sync"
)

// ConnectionGraph is a struct used for connected devices info
type ConnectionGraph struct {
	data  map[string]string
	mutex sync.RWMutex
}

// NewRssiMap will return newly init struct
func NewConnectionGraph() *ConnectionGraph {
	return &ConnectionGraph{data: map[string]string{}, mutex: sync.RWMutex{}}
}

// NewConnectionGraphFromRaw will return newly init struct
func NewConnectionGraphFromRaw(raw map[string]string) *ConnectionGraph {
	cg := NewConnectionGraph()
	for src, dst := range raw {
		cg.Set(src, dst)
	}
	return cg
}

// Data will return serialized form of struct as bytes
func (cg *ConnectionGraph) Data() ([]byte, error) {
	return encode(cg.GetAll())
}

// Set will update the map
func (cg *ConnectionGraph) Set(src, new string) {
	src = strings.ToUpper(src)
	new = strings.ToUpper(new)
	cg.mutex.Lock()
	cg.data[src] = new
	cg.mutex.Unlock()
}

// GetAll will get all data from map
func (cg *ConnectionGraph) GetAll() map[string]string {
	return cg.data
}

// Get will get from map
func (cg *ConnectionGraph) Get(src string) (string, bool) {
	src = strings.ToUpper(src)
	cg.mutex.RLock()
	ret, ok := cg.data[src]
	cg.mutex.RUnlock()
	return ret, ok
}

// Merge will write input map to this map
func (cg *ConnectionGraph) Merge(o *ConnectionGraph) {
	for addr := range o.data {
		val, _ := o.Get(addr)
		cg.Set(addr, val)
	}
}

// String returns json string of data
func (cg *ConnectionGraph) String() string {
	b, _ := json.Marshal(cg.GetAll())
	return string(b)
}

// GetConnectionGraphFromBytes constructs the map from bytes
func GetConnectionGraphFromBytes(data []byte) (*ConnectionGraph, error) {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	var m map[string]string
	err := dec.Decode(&m)
	if err != nil {
		return nil, err
	}
	return NewConnectionGraphFromRaw(m), nil
}
