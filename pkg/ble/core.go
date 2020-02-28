package ble

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
	"github.com/go-ble/ble/linux/hci/cmd"
	"github.com/pkg/errors"
)

type coreMethods interface {
	Stop() error
	SetDefaultDevice() error
	Connect(context.Context, ble.AdvFilter) (ble.Client, error)
	Dial(context.Context, ble.Addr) (ble.Client, error)
	Scan(context.Context, bool, ble.AdvHandler, ble.AdvFilter) error
	AdvertiseNameAndServices(context.Context, string, ...ble.UUID) error
	AddService(*ble.Service) error
}

type realCoreMethods struct{}

func (bc *realCoreMethods) Connect(ctx context.Context, f ble.AdvFilter) (ble.Client, error) {
	var client ble.Client
	err := util.CatchErrs(func() error {
		c, e := ble.Connect(ctx, f)
		client = c
		return e
	})
	return client, err

}

func (bc *realCoreMethods) Dial(ctx context.Context, addr ble.Addr) (ble.Client, error) {
	var client ble.Client
	err := util.CatchErrs(func() error {
		c, e := ble.Dial(ctx, addr)
		client = c
		return e
	})
	return client, err
}

func (bc *realCoreMethods) Scan(ctx context.Context, b bool, h ble.AdvHandler, f ble.AdvFilter) error {
	return util.CatchErrs(func() error {
		return ble.Scan(ctx, b, h, f)
	})
}

func (bc *realCoreMethods) Stop() error {
	err := util.CatchErrs(func() error {
		return ble.Stop()
	})
	if err != nil && err.Error() != "default device is not set" {
		return bc.resetHCI()
	}
	return nil
}

func (bc *realCoreMethods) resetHCI() error {
	out, err := exec.Command("hciconfig", "hci0", "reset").Output()
	if err != nil {
		return errors.Wrap(err, "HCI RESET FAILURE")
	}
	fmt.Println("HCI RESET COMPLETE: " + string(out))
	return nil
}

func (bc *realCoreMethods) AdvertiseNameAndServices(ctx context.Context, name string, uuids ...ble.UUID) error {
	return util.CatchErrs(func() error {
		return ble.AdvertiseNameAndServices(ctx, name, uuids...)
	})
}

func (bc *realCoreMethods) AddService(s *ble.Service) error {
	return util.CatchErrs(func() error {
		return ble.AddService(s)
	})
}

func (bc *realCoreMethods) newLinuxDevice() (ble.Device, error) {
	opts := cmd.LECreateConnection{
		LEScanInterval:        0x0060,    // 0x0004 - 0x4000; N * 0.625 msec
		LEScanWindow:          0x0060,    // 0x0004 - 0x4000; N * 0.625 msec
		InitiatorFilterPolicy: 0x00,      // White list is not used
		PeerAddressType:       0x00,      // Public Device Address
		PeerAddress:           [6]byte{}, //
		OwnAddressType:        0x00,      // Public Device Address
		ConnIntervalMin:       0x0028,    // 0x0006 - 0x0C80; N * 1.25 msec
		ConnIntervalMax:       0x0038,    // 0x0006 - 0x0C80; N * 1.25 msec
		ConnLatency:           0x0000,    // 0x0000 - 0x01F3; N * 1.25 msec
		SupervisionTimeout:    0x002A,    // 0x000A - 0x0C80; N * 10 msec
		MinimumCELength:       0x0000,    // 0x0000 - 0xFFFF; N * 0.625 msec
		MaximumCELength:       0x0000,    // 0x0000 - 0xFFFF; N * 0.625 msec
	}
	return linux.NewDevice(ble.OptConnParams(opts))
}

func (bc *realCoreMethods) SetDefaultDevice() error {
	return retry(func() error {
		err := util.CatchErrs(func() error {
			device, err := bc.newLinuxDevice()
			if err != nil {
				return err
			}
			ble.SetDefaultDevice(device)
			return nil
		})
		if err != nil {
			return bc.resetHCI()
		}
		return nil
	})
}
