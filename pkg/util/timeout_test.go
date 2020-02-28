package util

import (
	"errors"
	"testing"
	"time"

	"gotest.tools/assert"
)

func TestTimeout(t *testing.T) {
	x := time.Second * 3
	err := Timeout(func() error {
		time.Sleep(x + time.Second)
		return errors.New("should not get called")
	}, x)
	assert.ErrorContains(t, err, "Timeout")
}
