package util

import (
	"errors"
	"fmt"
	"time"
)

// Timeout is a utility method used to timeout function calls after the specified interval
func Timeout(fn func() error, duration time.Duration) error {
	err := make(chan error, 1)
	go func() {
		e := fn()
		err <- e
		if e != nil {
			fmt.Println("OOOF ERR: " + e.Error())
		}
	}()
	select {
	case ret := <-err:
		close(err)
		if ret != nil {
			fmt.Println("RAW ERR: " + ret.Error())
		}
		return ret
	case <-time.After(duration):
		close(err)
		fmt.Println("TIMEOUT ERR")
		return errors.New("Timeout")
	}
}
