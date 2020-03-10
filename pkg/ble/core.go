package ble

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
	"github.com/go-ble/ble/linux/hci/cmd"
	"github.com/pkg/errors"
)

const (
	hciResetDelay = 10 * time.Second
)

type coreMethods interface {
	Stop() error
	SetDefaultDevice() error
	Find(time.Duration, ble.Addr) (bool, error)
	Connect(time.Duration, ble.AdvFilter) (ble.Client, error)
	Dial(time.Duration, ble.Addr) (ble.Client, error)
	Scan(context.Context, ble.AdvHandler, ble.AdvFilter) error
	AdvertiseNameAndServices(string, ...ble.UUID) error
	AddService(*ble.Service) error
}

type realCoreMethods struct{}

func (bc *realCoreMethods) Connect(timeout time.Duration, f ble.AdvFilter) (ble.Client, error) {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	var client ble.Client
	err := util.CatchErrs(func() error {
		c, e := ble.Connect(ctx, f)
		client = c
		return e
	})
	return client, err
}

func (bc *realCoreMethods) Find(timeout time.Duration, addr ble.Addr) (bool, error) {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	var advs []ble.Advertisement
	err := util.CatchErrs(func() error {
		var e error
		advs, e = ble.Find(ctx, true, func(a ble.Advertisement) bool { return util.AddrEqualAddr(a.Addr().String(), addr.String()) })
		return e
	})
	return len(advs) > 0, err
}

func (bc *realCoreMethods) Dial(timeout time.Duration, addr ble.Addr) (ble.Client, error) {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
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

func (bc *realCoreMethods) Stop() error {
	err := util.CatchErrs(func() error {
		return ble.Stop()
	})
	if err != nil && err.Error() != "default device is not set" {
		return err
	}
	return nil
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

func isHCIDown() bool {
	out, _ := exec.Command("hciconfig").Output()
	resp := string(out)
	return strings.Contains(resp, "DOWN")
}

func (bc *realCoreMethods) SetDefaultDevice() error {
	return retry(func(attempts int) error {
		return util.CatchErrs(func() error {
			device, err := bc.newLinuxDevice()
			if err != nil {
				if isHCIDown() {
					forcePanic(errors.Wrap(err, "HCI is DOWN!"))
				}
				return errors.Wrap(err, fmt.Sprintf("newLinuxDevice issue (tried %d times)", attempts))
			}
			ble.SetDefaultDevice(device)
			return nil
		})
	})
}
