package ble

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
	"github.com/pkg/errors"
)

const (
	maxRetryAttempts = 5 // TODO: remove me and make infinite
)

type connectionListener interface {
	OnConnected(string, int)
	OnDisconnected()
}

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
	return util.CatchErrs(func() error {
		return ble.Stop()
	})
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

func (bc *realCoreMethods) SetDefaultDevice() error {
	return util.CatchErrs(func() error {
		device, err := linux.NewDevice()
		if err != nil {
			return err
		}
		ble.SetDefaultDevice(device)
		return nil
	})
}

type Connection interface {
	GetConnectedAddr() string
	GetRssiMap() *models.RssiMap
	Connect(ble.AdvFilter) error
	Dial(string) error
	Scan(func(ble.Advertisement)) error
	ScanForDuration(time.Duration, func(ble.Advertisement)) error
	ReadValue(string) ([]byte, error)
	WriteValue(string, []byte) error
	Context() context.Context
}

type ServiceInfo struct {
	Service     *ble.Service
	ServiceName string
	UUID        ble.UUID
}

type RealConnection struct {
	srcAddr         string
	connectedAddr   string
	rssiMap         *models.RssiMap
	secret          string
	cln             ble.Client
	serviceInfo     *ServiceInfo
	methods         coreMethods
	characteristics map[string]*ble.Characteristic
	mutex           *sync.Mutex
	listener        connectionListener
	ctx             context.Context
}

func (c *RealConnection) resetDevice() error {
	c.methods.Stop() // ignore error since it only yields error for empty device
	err := c.methods.SetDefaultDevice()
	if err != nil {
		return err
	}
	if c.serviceInfo == nil {
		return nil
	}
	err = c.methods.AddService(c.serviceInfo.Service)
	if err != nil {
		return err
	}
	go func() {
		c.methods.AdvertiseNameAndServices(c.ctx, c.serviceInfo.ServiceName, c.serviceInfo.UUID)
	}()
	return nil
}

func newRealConnection(addr string, secret string, listener connectionListener, methods coreMethods, serviceInfo *ServiceInfo) (*RealConnection, error) {
	conn := &RealConnection{
		srcAddr: addr, rssiMap: models.NewRssiMap(),
		secret: secret, mutex: &sync.Mutex{},
		methods: methods, characteristics: map[string]*ble.Characteristic{},
		listener: listener, serviceInfo: serviceInfo, ctx: util.MakeINFContext(),
	}
	if err := conn.resetDevice(); err != nil {
		return nil, err
	}
	return conn, nil
}

func NewRealConnection(addr string, secret string, listener connectionListener, serviceInfo *ServiceInfo) (*RealConnection, error) {
	return newRealConnection(addr, secret, listener, &realCoreMethods{}, serviceInfo)
}

func retry(fn func() error) error {
	err := errors.New("not error")
	attempts := 0
	for err != nil && attempts < maxRetryAttempts {
		if attempts > 0 {
			fmt.Printf("Error: %s\n Retrying...\n", err.Error())
		}
		err = fn()
		attempts += 1
	}
	if err != nil {
		return errors.Wrap(err, "Exceeded attempts issue: ")
	}
	return nil
}

func retryAndOptimize(c *RealConnection, fn func() error, reconnect bool) error {
	return retry(func() error {
		err := util.Optimize(fn)
		if err != nil {
			e := c.resetDevice()
			if e != nil {
				return errors.Wrap(e, " AND "+err.Error())
			}
			if reconnect {
				e := c.Dial(c.connectedAddr)
				if e != nil {
					return errors.Wrap(e, " AND "+err.Error())
				}
			}
		}
		return err
	})
}

func (c *RealConnection) updateRssiMap(a ble.Advertisement) {
	addr := a.Addr().String()
	rssi := a.RSSI()
	c.rssiMap.Set(c.srcAddr, addr, rssi)
}

func (c *RealConnection) Context() context.Context    { return c.ctx }
func (c *RealConnection) GetConnectedAddr() string    { return c.connectedAddr }
func (c *RealConnection) GetRssiMap() *models.RssiMap { return c.rssiMap }

type connnectOrDialHelper func() (ble.Client, string, error)

func (c *RealConnection) wrapConnectOrDial(fn connnectOrDialHelper) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.connectedAddr = ""
	err := retryAndOptimize(c, func() error {
		if c.cln != nil {
			c.cln.CancelConnection()
		}
		cln, addr, err := fn()
		c.cln = cln
		if err != nil {
			return errors.Wrap(err, "ConnectOrDial issue: ")
		}
		return c.handleCln(cln, addr)
	}, false)
	if err != nil && c.cln != nil {
		c.cln.CancelConnection()
	}
	return err
}

func (c *RealConnection) handleCln(cln ble.Client, addr string) error {
	go func() {
		<-cln.Disconnected()
		c.listener.OnDisconnected()
	}()
	_, err := cln.ExchangeMTU(util.MTU)
	if err != nil {
		return errors.Wrap(err, "ExchangeMTU issue: ")
	}
	p, err := cln.DiscoverProfile(true)
	if err != nil {
		return errors.Wrap(err, "DiscoverProfile issue: ")
	}
	rssi := cln.ReadRSSI()
	for _, s := range p.Services {
		if util.UuidEqualStr(s.UUID, util.MainServiceUUID) {
			for _, char := range s.Characteristics {
				uuid := util.UuidToStr(char.UUID)
				c.characteristics[uuid] = char
			}
			c.connectedAddr = addr
			c.listener.OnConnected(addr, rssi)
			return nil
		}
	}
	return errors.New("Could not find MainServiceUUID in broadcasted services")
}

func (c *RealConnection) Connect(filter ble.AdvFilter) error {
	return c.wrapConnectOrDial(func() (ble.Client, string, error) {
		var addr string
		cln, err := c.methods.Connect(c.ctx, func(a ble.Advertisement) bool {
			c.updateRssiMap(a)
			b := filter(a)
			if b {
				addr = a.Addr().String()
			}
			return b
		})
		return cln, addr, err
	})
}

func (c *RealConnection) Dial(addr string) error {
	return c.wrapConnectOrDial(func() (ble.Client, string, error) {
		cln, err := c.methods.Dial(c.ctx, ble.NewAddr(addr))
		return cln, addr, err
	})
}

func (c *RealConnection) scan(ctx context.Context, handle func(ble.Advertisement)) error {
	return c.methods.Scan(ctx, true, func(a ble.Advertisement) {
		c.updateRssiMap(a)
		handle(a)
	}, nil)
}

func (c *RealConnection) Scan(handle func(ble.Advertisement)) error {
	return c.scan(c.ctx, handle)
}

func (c *RealConnection) ScanForDuration(duration time.Duration, handle func(ble.Advertisement)) error {
	ctx, _ := context.WithTimeout(c.ctx, duration)
	return c.scan(ctx, handle)
}

func (c *RealConnection) getCharacteristic(uuid string) (*ble.Characteristic, error) {
	if c, ok := c.characteristics[uuid]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("No such uuid (%s) in characteristics (%v) advertised from server.", uuid, c.characteristics)
}

func (c *RealConnection) ReadValue(uuid string) ([]byte, error) {
	char, err := c.getCharacteristic(uuid)
	if err != nil {
		return nil, err
	}
	var encData []byte
	err = retryAndOptimize(c, func() error {
		dat, e := c.cln.ReadLongCharacteristic(char)
		encData = dat
		return errors.Wrap(e, "ReadLongCharacteristic issue: ")
	}, true)
	if err != nil {
		return nil, err
	}
	return util.Decrypt(encData, c.secret)
}

func (c *RealConnection) WriteValue(uuid string, data []byte) error {
	if data == nil || len(data) == 0 {
		return errors.New("Empty data provided. Will skip writing.")
	}
	char, err := c.getCharacteristic(uuid)
	if err != nil {
		return err
	}
	packets, err := util.EncodeDataAsPackets(data, c.secret)
	for _, packet := range packets {
		e := retryAndOptimize(c, func() error {
			return c.cln.WriteCharacteristic(char, packet, true)
		}, true)
		if e != nil {
			err = errors.Wrap(e, "WriteCharacteristic issue: ")
		}
	}
	return err
}
