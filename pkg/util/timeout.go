package util

import (
	"errors"
	"time"
)

// Timeout is a utility method used to timeout function calls after the specified interval
func Timeout(fn func() error, duration time.Duration) error {
	err := make(chan error, 1)
	closed := false
	defer func() {
		close(err)
		closed = true
	}()
	go func() {
		e := fn()
		if !closed {
			err <- e
		}
	}()
	select {
	case ret := <-err:
		return ret
	case <-time.After(duration):
		return errors.New("Timeout")
	}
}
