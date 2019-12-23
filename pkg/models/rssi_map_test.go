package models

import (
	"testing"

	"gotest.tools/assert"
)

func TestSetter(t *testing.T) {
	x := NewRssiMap()
	x.Set("a", "b", 1)
	assert.DeepEqual(t, x.data, map[string]map[string]int{"a": map[string]int{"b": 1}})
}

func TestMerge(t *testing.T) {
	a := "a"
	b := "b"
	x := NewRssiMap()
	y := NewRssiMap()
	z := NewRssiMap()
	x.Set(a, a, 1)
	x.Set(a, b, 2)
	y.Set(b, a, 1)
	y.Set(b, b, 2)
	z.Set(a, a, 1)
	z.Set(a, b, 2)
	z.Set(b, a, 1)
	z.Set(b, b, 2)
	x.Merge(&y)
	assert.DeepEqual(t, x.data, z.data)
}
