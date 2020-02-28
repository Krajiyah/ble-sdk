package ble

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
	"github.com/pkg/errors"
)

const (
	maxRetryAttempts = 5
	connectTimeout   = 5 * time.Second
)

type connectionListener interface {
	OnConnected(string, int)
	OnDisconnected()
}

type coreMethods interface {
	Connect(context.Context, ble.AdvFilter) (ble.Client, error)
	Dial(context.Context, ble.Addr) (ble.Client, error)
	Scan(context.Context, bool, ble.AdvHandler, ble.AdvFilter) error
}

type realCoreMethods struct{}

func (bc *realCoreMethods) Connect(ctx context.Context, f ble.AdvFilter) (ble.Client, error) {
	return ble.Connect(ctx, f)
}

func (bc *realCoreMethods) Dial(ctx context.Context, addr ble.Addr) (ble.Client, error) {
	return ble.Dial(ctx, addr)
}

func (bc *realCoreMethods) Scan(ctx context.Context, b bool, h ble.AdvHandler, f ble.AdvFilter) error {
	return ble.Scan(ctx, b, h, f)
}

type Connection interface {
	GetConnectedAddr() string
	GetRssiMap() *models.RssiMap
	Connect(context.Context, ble.AdvFilter) error
	Dial(context.Context, string) error
	Scan(context.Context, func(ble.Advertisement)) error
	ScanForDuration(context.Context, time.Duration, func(ble.Advertisement)) error
	ReadValue(string) ([]byte, error)
	WriteValue(string, []byte) error
}

type RealConnection struct {
	srcAddr         string
	connectedAddr   string
	rssiMap         *models.RssiMap
	secret          string
	cln             *ble.Client
	methods         coreMethods
	characteristics map[string]*ble.Characteristic
	mutex           *sync.Mutex
	listener        connectionListener
}

func NewRealConnection(addr string, secret string, listener connectionListener) *RealConnection {
	return &RealConnection{
		srcAddr: addr, rssiMap: models.NewRssiMap(),
		secret: secret, mutex: &sync.Mutex{},
		methods: &realCoreMethods{}, characteristics: map[string]*ble.Characteristic{},
		listener: listener,
	}
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

func retryAndOptimize(fn func() error) error { return retry(func() error { return util.Optimize(fn) }) }

func retryAndCatch(fn func() error) error { return retry(func() error { return util.CatchErrs(fn) }) }

func (c *RealConnection) updateRssiMap(a ble.Advertisement) {
	addr := a.Addr().String()
	rssi := a.RSSI()
	c.rssiMap.Set(c.srcAddr, addr, rssi)
}

func (c *RealConnection) GetConnectedAddr() string    { return c.connectedAddr }
func (c *RealConnection) GetRssiMap() *models.RssiMap { return c.rssiMap }

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

type connnectOrDialHelper func() (ble.Client, string, error)

func (c *RealConnection) wrapConnectOrDial(fn connnectOrDialHelper) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	err := retryAndCatch(func() error {
		if c.cln != nil {
			c.connectedAddr = ""
			(*c.cln).CancelConnection()
		}
		cln, addr, err := fn()
		c.cln = &cln
		if err != nil {
			return errors.Wrap(err, "coreMethods ConnectOrDial issue: ")
		}
		return c.handleCln(cln, addr)
	})
	if err != nil && c.cln != nil {
		(*c.cln).CancelConnection()
	}
	return err
}

func (c *RealConnection) Connect(ctx context.Context, filter ble.AdvFilter) error {
	return c.wrapConnectOrDial(func() (ble.Client, string, error) {
		var addr string
		ctx, _ := context.WithTimeout(context.Background(), time.Second*10)
		cln, err := c.methods.Connect(ctx, func(a ble.Advertisement) bool {
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

func (c *RealConnection) Dial(ctx context.Context, addr string) error {
	return c.wrapConnectOrDial(func() (ble.Client, string, error) {
		ctx, _ := context.WithTimeout(context.Background(), time.Second*10)
		cln, err := c.methods.Dial(ctx, ble.NewAddr(addr))
		return cln, addr, err
	})
}

func (c *RealConnection) Scan(ctx context.Context, handle func(ble.Advertisement)) error {
	return c.methods.Scan(ctx, true, func(a ble.Advertisement) {
		c.updateRssiMap(a)
		handle(a)
	}, nil)
}

func (c *RealConnection) ScanForDuration(ctx context.Context, duration time.Duration, handle func(ble.Advertisement)) error {
	ctx, _ = context.WithTimeout(ctx, duration)
	return c.Scan(ctx, handle)
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
	err = retryAndOptimize(func() error {
		dat, e := (*c.cln).ReadLongCharacteristic(char)
		encData = dat
		return errors.Wrap(e, "ReadLongCharacteristic issue: ")
	})
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
		e := retryAndOptimize(func() error {
			return (*c.cln).WriteCharacteristic(char, packet, true)
		})
		if e != nil {
			err = errors.Wrap(e, "WriteCharacteristic issue: ")
		}
	}
	return err
}
