package util

import (
	"testing"

	"github.com/Krajiyah/ble-sdk/pkg/models"
	"gotest.tools/assert"
)

func TestShortestPathCase1(t *testing.T) {
	src := "A"
	dst := "B"
	middle := "C"
	rssiMap := models.RssiMap{
		src: map[string]int{
			dst:    3,
			middle: 2,
		},
		middle: map[string]int{
			dst: 5,
		},
	}
	path, err := ShortestPath(rssiMap, src, dst)
	assert.NilError(t, err)
	assert.DeepEqual(t, path, []string{src, dst})
}

func TestShortestPathCase2(t *testing.T) {
	src := "A"
	dst := "B"
	middle := "C"
	rssiMap := models.RssiMap{
		src: map[string]int{
			dst:    70,
			middle: 2,
		},
		middle: map[string]int{
			dst: 3,
		},
	}
	path, err := ShortestPath(rssiMap, src, dst)
	assert.NilError(t, err)
	assert.DeepEqual(t, path, []string{src, middle, dst})
}

func TestShortestPathCase3(t *testing.T) {
	a := "A"
	b := "B"
	c := "C"
	d := "D"
	e := "E"
	f := "F"
	rssiMap := models.RssiMap{
		a: map[string]int{
			c: 1,
			d: 2,
		},
		c: map[string]int{
			e: 4,
		},
		e: map[string]int{
			b: 200,
		},
		d: map[string]int{
			f: 3,
		},
		f: map[string]int{
			b: 1,
		},
	}
	path, err := ShortestPath(rssiMap, a, b)
	assert.NilError(t, err)
	assert.DeepEqual(t, path, []string{a, d, f, b})
}
