package ble

import (
	"context"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
	"github.com/pkg/errors"
)

type coreMethods interface {
	SetDefaultDevice(time.Duration) error
	Connect(time.Duration, ble.AdvFilter) (ble.Client, error)
	Dial(time.Duration, ble.Addr) (ble.Client, error)
	Scan(context.Context, ble.AdvHandler, ble.AdvFilter) error
	AdvertiseNameAndServices(string, ...ble.UUID) error
	AddService(*ble.Service) error
}

type realCoreMethods struct{}

func (bc *realCoreMethods) Connect(timeout time.Duration, f ble.AdvFilter) (ble.Client, error) {
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), timeout))
	var client ble.Client
	err := util.CatchErrs(func() error {
		c, e := ble.Connect(ctx, f)
		client = c
		return e
	})
	return client, err
}

func (bc *realCoreMethods) Dial(timeout time.Duration, addr ble.Addr) (ble.Client, error) {
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), timeout))
	var client ble.Client
	err := util.CatchErrs(func() error {
		c, e := ble.Dial(ctx, addr)
		client = c
		return e
	})
	return client, err
}

func (bc *realCoreMethods) Scan(ctx context.Context, h ble.AdvHandler, f ble.AdvFilter) error {
	return util.CatchErrs(func() error {
		return ble.Scan(ctx, true, h, f)
	})
}

func (bc *realCoreMethods) AdvertiseNameAndServices(name string, uuids ...ble.UUID) error {
	return util.CatchErrs(func() error {
		return ble.AdvertiseNameAndServices(util.MakeINFContext(), name, uuids...)
	})
}

func (bc *realCoreMethods) AddService(s *ble.Service) error {
	return util.CatchErrs(func() error {
		return ble.AddService(s)
	})
}

func (bc *realCoreMethods) newLinuxDevice(timeout time.Duration) (ble.Device, error) {
	opts := []ble.Option{
		ble.OptDialerTimeout(timeout), // client to server timeout
	}
	return linux.NewDevice(opts...)
}

func (bc *realCoreMethods) SetDefaultDevice(timeout time.Duration) error {
	device, err := bc.newLinuxDevice(timeout)
	if err != nil {
		return errors.Wrap(err, "newLinuxDevice issue")
	}
	ble.SetDefaultDevice(device)
	return nil
}
