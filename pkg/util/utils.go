package util

import (
	"context"
	"strings"
	"time"

	"github.com/currantlabs/ble"
)

const (
	inf     = 1000000
	timeout = time.Second * 5
)

func AddrEqualAddr(a string, b string) bool {
	return strings.ToUpper(a) == strings.ToUpper(b)
}

func UuidEqualStr(u ble.UUID, s string) bool {
	compare := strings.Replace(s, "-", "", -1)
	return AddrEqualAddr(compare, u.String())
}

func MakeINFContext() context.Context {
	return ble.WithSigHandler(context.WithTimeout(context.Background(), inf*time.Hour))
}

func Optimize(fn func() error) error {
	return Timeout(func() error {
		var err error
		TryCatchBlock{
			Try: func() {
				err = fn()
			},
			Catch: func(e error) {
				err = e
			},
		}.Do()
		return err
	}, timeout)
}
