package ble

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
)

func (c *RealConnection) resetDevice() error {
	c.resetDeviceMutex.Lock()
	defer c.resetDeviceMutex.Unlock()
	err := c.methods.Stop()
	time.Sleep(stopDelay)
	if err != nil {
		fmt.Println("Warning: Stop Issue: " + err.Error())
	}
	err = c.methods.SetDefaultDevice(c.timeout)
	time.Sleep(setDefaultDeviceDelay)
	if err != nil {
		return errors.Wrap(err, "SetDefaultDevice issue")
	}
	if c.serviceInfo == nil {
		return nil
	}
	err = c.methods.AddService(c.serviceInfo.Service)
	if err != nil {
		return err
	}
	go func() {
		c.methods.AdvertiseNameAndServices(c.serviceInfo.ServiceName, c.serviceInfo.UUID)
	}()
	return nil
}
