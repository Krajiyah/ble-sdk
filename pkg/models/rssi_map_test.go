package models

import (
	"testing"

	"gotest.tools/assert"
)

func TestSetter(t *testing.T) {
	x := RssiMap{}
	x.Set("a", "b", 1)
	assert.DeepEqual(t, x, RssiMap{"a": map[string]int{"b": 1}})
}

func TestMerge(t *testing.T) {
	a := "a"
	b := "b"
	x := RssiMap{
		a: {
			a: 1,
			b: 2,
		},
	}
	y := RssiMap{
		b: {
			a: 1,
			b: 2,
		},
	}
	z := RssiMap{
		a: {
			a: 1,
			b: 2,
		},
		b: {
			a: 1,
			b: 2,
		},
	}
	x.Merge(&y)
	assert.DeepEqual(t, x, z)
}
