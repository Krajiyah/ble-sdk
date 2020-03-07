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
	stopDelay             = time.Second * 3
	setDefaultDeviceDelay = time.Second * 2
	maxRetryAttempts      = 6
)

type connectionListener interface {
	OnConnected(string)
	OnDisconnected()
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
	srcAddr          string
	connectedAddr    string
	rssiMap          *models.RssiMap
	secret           string
	cln              ble.Client
	serviceInfo      *ServiceInfo
	methods          coreMethods
	characteristics  map[string]*ble.Characteristic
	connectionMutex  *sync.Mutex
	resetDeviceMutex *sync.Mutex
	listener         connectionListener
	ctx              context.Context
}

func (c *RealConnection) resetDevice() error {
	c.resetDeviceMutex.Lock()
	defer c.resetDeviceMutex.Unlock()
	err := c.methods.Stop()
	time.Sleep(stopDelay)
	if err != nil {
		fmt.Println("Warning: Stop Issue: " + err.Error())
	}
	err = c.methods.SetDefaultDevice()
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
		c.methods.AdvertiseNameAndServices(c.ctx, c.serviceInfo.ServiceName, c.serviceInfo.UUID)
	}()
	return nil
}

func newRealConnection(addr string, secret string, listener connectionListener, methods coreMethods, serviceInfo *ServiceInfo) (*RealConnection, error) {
	conn := &RealConnection{
		srcAddr: addr, rssiMap: models.NewRssiMap(),
		secret: secret, connectionMutex: &sync.Mutex{}, resetDeviceMutex: &sync.Mutex{},
		methods: methods, characteristics: map[string]*ble.Characteristic{},
		listener: listener, serviceInfo: serviceInfo, ctx: util.MakeINFContext(),
	}
	if err := conn.resetDevice(); err != nil {
		return nil, errors.Wrap(err, "newRealConnection resetDevice issue")
	}
	return conn, nil
}

func NewRealConnection(addr string, secret string, listener connectionListener, serviceInfo *ServiceInfo) (*RealConnection, error) {
	return newRealConnection(addr, secret, listener, &realCoreMethods{}, serviceInfo)
}

func retry(fn func(int) error) error {
	err := errors.New("not error")
	attempts := 0
	for err != nil && attempts < maxRetryAttempts {
		if attempts > 0 {
			fmt.Printf("Error: %s\n Retrying...\n", err.Error())
		}
		attempts += 1
		err = fn(attempts)
	}
	if err != nil {
		return errors.Wrap(err, "Exceeded attempts issue: ")
	}
	return nil
}

type retryAndOptimizeError struct {
	method    string
	attempt   int
	doesReset bool
	doesDial  bool
	original  error
	reset     error
	dial      error
}

func (err *retryAndOptimizeError) Error() error {
	original := err.original.Error()
	const pass = "✔"
	resetDevice := "n/a"
	reconnect := "n/a"
	if err.reset != nil {
		resetDevice = err.reset.Error()
	} else if err.doesReset {
		resetDevice = pass
	}
	if err.dial != nil {
		reconnect = err.dial.Error()
	} else if err.doesDial {
		reconnect = pass
	}
	return errors.New(fmt.Sprintf(`
-------
Method: %s
Attempt: %d
Original: %s
ResetDevice: %s
Reconnect: %s
-------`, err.method, err.attempt, original, resetDevice, reconnect))
}

func retryAndOptimize(c *RealConnection, method string, fn func() error, reconnect bool) error {
	err := retry(func(attempts int) error {
		err := &retryAndOptimizeError{method: method, attempt: attempts}
		err.original = util.Optimize(fn)
		if err.original == nil {
			return nil
		}
		err.doesReset = true
		err.reset = c.resetDevice()
		if err.reset == nil {
			return err.Error()
		}
		if !reconnect {
			return err.Error()
		}
		fmt.Println("Reconnecting...")
		err.doesDial = true
		err.dial = c.Dial(c.connectedAddr)
		if err.dial == nil {
			return err.Error()
		}
		return err.Error()
	})
	if err != nil {
		forcePanic(err)
	}
	return nil
}

func forcePanic(err error) { panic(errors.New(util.ForcePanicMsgPrefix + err.Error())) }

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
	c.connectionMutex.Lock()
	defer c.connectionMutex.Unlock()
	err := retryAndOptimize(c, "ConnectOrDial", func() error {
		if c.cln != nil {
			c.cln.CancelConnection()
		}
		cln, addr, err := fn()
		c.cln = cln
		if err != nil {
			return err
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
	c.connectionMutex.Lock()
	defer c.connectionMutex.Unlock()
	char, err := c.getCharacteristic(uuid)
	if err != nil {
		return nil, err
	}
	var encData []byte
	err = retryAndOptimize(c, "ReadLongCharacteristic", func() error {
		dat, e := c.cln.ReadLongCharacteristic(char)
		encData = dat
		return e
	}, true)
	if err != nil {
		return nil, err
	}
	if len(encData) == 0 {
		return nil, errors.New("Received Empty Data")
	}
	return util.Decrypt(encData, c.secret)
}

func (c *RealConnection) WriteValue(uuid string, data []byte) error {
	c.connectionMutex.Lock()
	defer c.connectionMutex.Unlock()
	if data == nil || len(data) == 0 {
		return errors.New("Empty data provided. Will skip writing.")
	}
	char, err := c.getCharacteristic(uuid)
	if err != nil {
		return err
	}
	packets, err := util.EncodeDataAsPackets(data, c.secret)
	if err != nil {
		return err
	}
	for _, packet := range packets {
		err := retryAndOptimize(c, "WriteCharacteristic", func() error {
			return c.cln.WriteCharacteristic(char, packet, true)
		}, true)
		if err != nil {
			return err
		}
	}
	return nil
}
