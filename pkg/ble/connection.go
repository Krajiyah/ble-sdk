package ble

import (
	"context"
	"sync"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
	"github.com/pkg/errors"
)

const (
	maxRetryAttempts = 5
)

type connectionListener interface {
	OnConnected(string)
	OnDisconnected()
}

type Connection interface {
	GetConnectedAddr() string
	GetRssiMap() *models.RssiMap
	Connect(ble.AdvFilter)
	Disconnect() error
	Dial(string)
	Scan(func(ble.Advertisement)) error
	ScanForDuration(time.Duration, func(ble.Advertisement)) error
	CollectAdvs(time.Duration) ([]ble.Advertisement, error)
	ReadValue(string) ([]byte, error)
	BlockingWriteValue(string, []byte) error
	NonBlockingWriteValue(string, []byte)
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
	connectionMutex *sync.Mutex
	listener        connectionListener
	timeout         time.Duration
}

func (c *RealConnection) setupDevice() error {
	err := c.methods.SetDefaultDevice(c.timeout)
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

func newRealConnection(addr string, secret string, timeout time.Duration, listener connectionListener, methods coreMethods, serviceInfo *ServiceInfo) (*RealConnection, error) {
	conn := &RealConnection{
		srcAddr: addr, rssiMap: models.NewRssiMap(),
		secret: secret, connectionMutex: &sync.Mutex{},
		methods: methods, characteristics: map[string]*ble.Characteristic{},
		listener: listener, serviceInfo: serviceInfo,
		timeout: timeout,
	}
	if err := conn.setupDevice(); err != nil {
		return nil, errors.Wrap(err, "newRealConnection resetDevice issue")
	}
	return conn, nil
}

func NewRealConnection(addr string, secret string, timeout time.Duration, listener connectionListener, serviceInfo *ServiceInfo) (*RealConnection, error) {
	return newRealConnection(addr, secret, timeout, listener, &realCoreMethods{}, serviceInfo)
}

func (c *RealConnection) updateRssiMap(a ble.Advertisement) {
	addr := a.Addr().String()
	rssi := a.RSSI()
	c.rssiMap.Set(c.srcAddr, addr, rssi)
}

func (c *RealConnection) GetConnectedAddr() string    { return c.connectedAddr }
func (c *RealConnection) GetRssiMap() *models.RssiMap { return c.rssiMap }

func (c *RealConnection) Connect(filter ble.AdvFilter) {
	c.wrapConnectOrDial(func() (ble.Client, string, error) {
		var addr string
		cln, err := c.methods.Connect(c.timeout, func(a ble.Advertisement) bool {
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

func (c *RealConnection) Dial(addr string) {
	c.wrapConnectOrDial(func() (ble.Client, string, error) {
		cln, err := c.methods.Dial(c.timeout, ble.NewAddr(addr))
		return cln, addr, err
	})
}

func (c *RealConnection) scan(ctx context.Context, handle func(ble.Advertisement)) error {
	return c.methods.Scan(ctx, func(a ble.Advertisement) {
		c.updateRssiMap(a)
		handle(a)
	}, nil)
}

func (c *RealConnection) Scan(handle func(ble.Advertisement)) error {
	ctx := ble.WithSigHandler(util.MakeINFContext(), func() {})
	return c.scan(ctx, handle)
}

func (c *RealConnection) CollectAdvs(duration time.Duration) ([]ble.Advertisement, error) {
	advs := &sync.Map{}
	err := c.ScanForDuration(duration, func(a ble.Advertisement) {
		advs.Store(a.Addr().String(), a)
	})
	if err != nil {
		return nil, err
	}
	ret := []ble.Advertisement{}
	advs.Range(func(_ interface{}, v interface{}) bool {
		ret = append(ret, v.(ble.Advertisement))
		return true
	})
	return ret, nil
}

func (c *RealConnection) ScanForDuration(timeout time.Duration, handle func(ble.Advertisement)) error {
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), timeout))
	err := c.scan(ctx, handle)
	if err != nil && err.Error() == "context deadline exceeded" {
		err = nil
	}
	return err
}
