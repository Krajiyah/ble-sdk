package ble

import (
	"fmt"

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

func retryAndPanic(c *RealConnection, method string, fn func() error) {
	err := retry(func() error {
		e := util.CatchErrs(fn)
		if e == nil {
			return nil
		}
		return errors.Wrap(e, method+" issue: ")
	})
	if err != nil {
		forcePanic(err)
	}
}
