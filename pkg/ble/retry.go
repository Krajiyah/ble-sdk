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
	method    string
	attempt   int
	doesReset bool
	doesDial  bool
	original  error
	reset     error
	dial      error
}

func (err *retryAndOptimizeError) Error() error {
	original := err.original.Error()
	const pass = "✔"
	resetDevice := "n/a"
	reconnect := "n/a"
	if err.reset != nil {
		resetDevice = err.reset.Error()
	} else if err.doesReset {
		resetDevice = pass
	}
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
ResetDevice: %s
Reconnect: %s
-------`, err.method, err.attempt, original, resetDevice, reconnect))
}

func retryAndPanic(c *RealConnection, method string, fn func(int) error) {
	err := retry(fn)
	if err != nil {
		forcePanic(err)
	}
}

func wrappedRetry(c *RealConnection, method string, fn func() error, reconnect bool) {
	retryAndPanic(c, method, func(attempts int) error {
		err := &retryAndOptimizeError{method: method, attempt: attempts}
		err.original = util.Optimize(fn, c.timeout)
		if err.original == nil {
			return nil
		}
		err.doesReset = true
		err.reset = c.resetDevice()
		if err.reset != nil || !reconnect {
			return err.Error()
		}
		fmt.Println("Reconnecting...")
		err.doesDial = true
		c.Dial(c.connectedAddr)
		return err.Error()
	})
}

func retryAndOptimizeConnectOrDial(c *RealConnection, method string, fn func() error) {
	wrappedRetry(c, method, fn, false)
}

func retryAndOptimizeReadOrWrite(c *RealConnection, method string, fn func() error) {
	wrappedRetry(c, method, fn, true)
}
