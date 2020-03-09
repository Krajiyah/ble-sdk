package ble

import (
	"fmt"

	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
	"github.com/pkg/errors"
)

type connnectOrDialHelper func() (ble.Client, string, error)

func (c *RealConnection) wrapConnectOrDial(fn connnectOrDialHelper) {
	c.connectionMutex.Lock()
	defer c.connectionMutex.Unlock()
	retryAndPanic(c, "ConnectOrDial", func(_ int) error {
		checkAndCancel(c.cln)
		cln, addr, err := c.initBLEClient(fn)
		if err != nil {
			return err
		}
		return c.completeBLEClient(cln, addr)
	})
}

func (c *RealConnection) initBLEClient(fn connnectOrDialHelper) (ble.Client, string, error) {
	var addr string
	var cln ble.Client
	err := util.CatchErrs(func() error {
		var e error
		cln, addr, e = fn()
		if e != nil {
			checkAndCancel(cln)
			return e
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	c.cln = cln
	go func() {
		<-cln.Disconnected() // TODO: is this slowing things down?
		c.listener.OnDisconnected()
	}()
	return cln, addr, nil
}

func (c *RealConnection) completeBLEClient(cln ble.Client, addr string) error {
	return util.CatchErrs(func() error {
		_, err := cln.ExchangeMTU(util.MTU)
		if err != nil {
			return errors.Wrap(err, "ExchangeMTU issue: ")
		}
		p, err := cln.DiscoverProfile(true)
		if err != nil {
			return errors.Wrap(err, "DiscoverProfile issue: ")
		}
		for _, s := range p.Services {
			if util.UuidEqualStr(s.UUID, util.MainServiceUUID) {
				for _, char := range s.Characteristics {
					uuid := util.UuidToStr(char.UUID)
					c.characteristics[uuid] = char
				}
				c.connectedAddr = addr
				c.listener.OnConnected(addr)
				return nil
			}
		}
		return errors.New("Could not find MainServiceUUID in broadcasted services")
	})
}

func checkAndCancel(cln ble.Client) {
	if cln != nil {
		fmt.Println("Would have cancelled connection...")
		// cln.CancelConnection()
	}
}
