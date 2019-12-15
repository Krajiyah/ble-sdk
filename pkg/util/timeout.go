package util

import (
	"errors"
	"time"
)

// Timeout is a utility method used to timeout function calls after the specified interval
func Timeout(fn func() error, duration time.Duration) error {
	err := make(chan error, 1)
	go func() {
		err <- fn()
	}()
	select {
	case ret := <-err:
		close(err)
		return ret
	case <-time.After(duration):
		close(err)
		return errors.New("Timeout")
	}
}
