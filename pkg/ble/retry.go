package ble

import (
	"fmt"

	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/pkg/errors"
)

func forcePanic(err error) { panic(errors.New(util.ForcePanicMsgPrefix + err.Error())) }

func retry(fn func(int) error) error {
	err := errors.New("not error")
	attempts := 0
	for err != nil && attempts < maxRetryAttempts {
		if attempts > 0 {
			fmt.Printf("Error: %s\n Retrying...\n", err.Error())
		}
		attempts += 1
		err = fn(attempts)
	}
	if err != nil {
		return errors.Wrap(err, "Exceeded attempts issue: ")
	}
	return nil
}

type retryAndOptimizeError struct {
	method   string
	attempt  int
	doesDial bool
	original error
	dial     error
}

func (err *retryAndOptimizeError) Error() error {
	original := err.original.Error()
	const pass = "✔"
	reconnect := "n/a"
	if err.dial != nil {
		reconnect = err.dial.Error()
	} else if err.doesDial {
		reconnect = pass
	}
	return errors.New(fmt.Sprintf(`
-------
Method: %s
Attempt: %d
Original: %s
Reconnect: %s
-------`, err.method, err.attempt, original, reconnect))
}

func retryAndPanic(c *RealConnection, method string, fn func() error, reconnect bool) {
	err := retry(func(attempts int) error {
		err := &retryAndOptimizeError{method: method, attempt: attempts}
		err.original = util.CatchErrs(fn)
		if err.original == nil {
			return nil
		}
		fmt.Println("Reconnecting...")
		err.doesDial = true
		c.Dial(c.connectedAddr)
		return err.Error()
	})
	if err != nil {
		forcePanic(err)
	}
}

func retryAndOptimizeConnectOrDial(c *RealConnection, method string, fn func() error) {
	retryAndPanic(c, method, fn, false)
}

func retryAndOptimizeReadOrWrite(c *RealConnection, method string, fn func() error) {
	retryAndPanic(c, method, fn, true)
}
