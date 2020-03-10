package ble

import (
	"fmt"
	"strings"

	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/pkg/errors"
)

func forcePanic(err error) { panic(errors.New(util.ForcePanicMsgPrefix + err.Error())) }

func retry(fn func() error) error {
	err := errors.New("not error")
	attempts := 0
	for err != nil && attempts < maxRetryAttempts {
		if attempts > 0 {
			fmt.Printf("Attempt: %d Error: %s\n Retrying...\n", attempts, err.Error())
		}
		attempts += 1
		err = fn()
	}
	if err != nil {
		return errors.Wrap(err, "Exceeded attempts issue: ")
	}
	return nil
}

func retryAndPanic(c *RealConnection, method string, fn func() error, reconnect bool) {
	err := retry(func() error {
		e := util.CatchErrs(fn)
		if e == nil {
			return nil
		}
		if reconnect && strings.Contains(e.Error(), "closed pipe") {
			fmt.Println("Issue with " + method + " so reconnecting...")
			c.Dial(c.connectedAddr)
		}
		return errors.Wrap(e, method+" issue: ")
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
