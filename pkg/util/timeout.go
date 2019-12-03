package util

import (
	"errors"
	"time"
)

// Timeout is a utility method used to timeout function calls after the specified interval
func Timeout(fn func() error, duration time.Duration) error {
	var err error
	ch := make(chan bool, 1)
	defer close(ch)
	go func() {
		err = fn()
		ch <- true
	}()
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ch:
		return err
	case <-timer.C:
		return errors.New("Timeout")
	}
}
