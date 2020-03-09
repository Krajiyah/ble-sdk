package ble

import (
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
	"github.com/pkg/errors"
)

type connnectOrDialHelper func() (ble.Client, string, error)

func (c *RealConnection) wrapConnectOrDial(fn connnectOrDialHelper) {
	c.connectionMutex.Lock()
	defer c.connectionMutex.Unlock()
	retryAndOptimizeConnectOrDial(c, "ConnectOrDial", func() error {
		checkAndCancel(c.cln)
		cln, addr, err := c.initBLEClient(fn)
		if err != nil {
			return err
		}
		return c.completeBLEClient(cln, addr)
	})
}

func (c *RealConnection) initBLEClient(fn connnectOrDialHelper) (ble.Client, string, error) {
	cln, addr, err := fn()
	if err != nil {
		checkAndCancel(cln)
		return nil, "", err
	}
	c.cln = cln
	go func() {
		<-cln.Disconnected()
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
		cln.CancelConnection()
	}
}
