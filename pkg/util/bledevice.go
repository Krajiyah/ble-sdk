package util

import (
	"runtime"

	"github.com/currantlabs/ble"
	"github.com/currantlabs/ble/darwin"
	"github.com/currantlabs/ble/linux"
)

// NewDevice will return ble.Device for correct OS
func NewDevice() (ble.Device, error) {
	if runtime.GOOS == "darwin" {
		return darwin.NewDevice()
	}
	return linux.NewDevice()
}
