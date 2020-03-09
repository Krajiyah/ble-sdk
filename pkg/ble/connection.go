package ble

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
	"github.com/google/uuid"
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
	Connect(ble.AdvFilter)
	Dial(string)
	Scan(func(ble.Advertisement)) error
	ScanForDuration(time.Duration, func(ble.Advertisement)) error
	ReadValue(string) ([]byte, error)
	BlockingWriteValue(string, []byte) error
	NonBlockingWriteValue(string, []byte)
	Context() context.Context
}

type ServiceInfo struct {
	Service     *ble.Service
	ServiceName string
	UUID        ble.UUID
}

type clnWrapper struct {
	cln  ble.Client
	guid string
}

type RealConnection struct {
	srcAddr          string
	connectedAddr    string
	rssiMap          *models.RssiMap
	secret           string
	cln              *clnWrapper
	serviceInfo      *ServiceInfo
	methods          coreMethods
	characteristics  map[string]*ble.Characteristic
	connectionMutex  *sync.Mutex
	resetDeviceMutex *sync.Mutex
	listener         connectionListener
	ctx              context.Context
	timeout          time.Duration
}

func (c *RealConnection) getClient(purpose string) ble.Client {
	wrapper := c.cln
	return wrapper.cln
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

func newRealConnection(addr string, secret string, timeout time.Duration, listener connectionListener, methods coreMethods, serviceInfo *ServiceInfo) (*RealConnection, error) {
	conn := &RealConnection{
		srcAddr: addr, rssiMap: models.NewRssiMap(),
		secret: secret, connectionMutex: &sync.Mutex{}, resetDeviceMutex: &sync.Mutex{},
		methods: methods, characteristics: map[string]*ble.Characteristic{},
		listener: listener, serviceInfo: serviceInfo, ctx: util.MakeINFContext(),
		timeout: timeout,
	}
	if err := conn.resetDevice(); err != nil {
		return nil, errors.Wrap(err, "newRealConnection resetDevice issue")
	}
	return conn, nil
}

func NewRealConnection(addr string, secret string, timeout time.Duration, listener connectionListener, serviceInfo *ServiceInfo) (*RealConnection, error) {
	return newRealConnection(addr, secret, timeout, listener, &realCoreMethods{}, serviceInfo)
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

func retryAndOptimize(c *RealConnection, method string, fn func() error, reconnect bool) {
	err := retry(func(attempts int) error {
		err := &retryAndOptimizeError{method: method, attempt: attempts}
		err.original = util.Optimize(fn, c.timeout)
		if err.original == nil {
			return nil
		}
		err.doesReset = true
		err.reset = c.resetDevice()
		if err.reset != nil || !reconnect {
			return err.Error()
		}
		fmt.Println("Reconnecting...")
		err.doesDial = true
		c.Dial(c.connectedAddr)
		return err.Error()
	})
	if err != nil {
		forcePanic(err)
	}
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

func (c *RealConnection) wrapConnectOrDial(fn connnectOrDialHelper) {
	c.connectionMutex.Lock()
	defer c.connectionMutex.Unlock()
	retryAndOptimize(c, "ConnectOrDial", func() error {
		if c.cln != nil {
			c.getClient("CancelConnection").CancelConnection()
		}
		cln, addr, err := fn()
		if err != nil {
			if cln != nil {
				c.getClient("CancelConnection").CancelConnection()
			}
			return err
		}
		c.cln = &clnWrapper{cln: cln, guid: uuid.New().String()}
		return c.handleCln(cln, addr)
	}, false)
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

func (c *RealConnection) Connect(filter ble.AdvFilter) {
	c.wrapConnectOrDial(func() (ble.Client, string, error) {
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

func (c *RealConnection) Dial(addr string) {
	c.wrapConnectOrDial(func() (ble.Client, string, error) {
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
	err := c.scan(ctx, handle)
	if err != nil && err.Error() == "context deadline exceeded" {
		err = nil
	}
	return err
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
	encDataBuff := make(chan []byte, 1)
	retryAndOptimize(c, "ReadLongCharacteristic", func() error {
		dat, e := c.getClient("ReadLongCharacteristic").ReadLongCharacteristic(char)
		if e != nil {
			return e
		}
		go func() { encDataBuff <- dat }()
		return nil
	}, true)
	encData := <-encDataBuff
	close(encDataBuff)
	if len(encData) == 0 {
		return nil, errors.New("Received Empty Data")
	}
	return util.Decrypt(encData, c.secret)
}

func (c *RealConnection) prepWriteValue(uuid string, data []byte) (*ble.Characteristic, [][]byte, error) {
	if data == nil || len(data) == 0 {
		return nil, nil, errors.New("empty data to write")
	}
	char, err := c.getCharacteristic(uuid)
	if err != nil {
		return nil, nil, err
	}
	packets, err := util.EncodeDataAsPackets(data, c.secret)
	if err != nil {
		return nil, nil, err
	}
	return char, packets, nil
}

func (c *RealConnection) doWrite(wg *sync.WaitGroup, char *ble.Characteristic, packet []byte) {
	retryAndOptimize(c, "WriteCharacteristic", func() error {
		return c.getClient("WriteCharacteristic").WriteCharacteristic(char, packet, true)
	}, true)
	wg.Done()
}

func (c *RealConnection) BlockingWriteValue(uuid string, data []byte) error {
	char, packets, err := c.prepWriteValue(uuid, data)
	if err != nil {
		return err
	}
	wg := &sync.WaitGroup{}
	for _, packet := range packets {
		wg.Add(1)
		go c.doWrite(wg, char, packet)
	}
	wg.Wait()
	return nil
}

func (c *RealConnection) NonBlockingWriteValue(uuid string, data []byte) {
	go func() {
		c.BlockingWriteValue(uuid, data)
	}()
}
