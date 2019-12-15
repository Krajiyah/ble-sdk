package util

import (
	"errors"
	"testing"
	"time"

	"gotest.tools/assert"
)

func TestTimeout(t *testing.T) {
	err := Timeout(func() error {
		time.Sleep(timeout + 2)
		return errors.New("should not get called")
	}, timeout)
	assert.ErrorContains(t, err, "Timeout")
}
