package forwarder

import (
	"errors"
	"time"

	"github.com/Krajiyah/ble-sdk/pkg/models"
	"github.com/Krajiyah/ble-sdk/pkg/server"
	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/currantlabs/ble"
	"github.com/currantlabs/ble/linux"
	"golang.org/x/net/context"
)

const (
	// MainServiceUUID represents UUID for ble service for all ble characteristics
	MainServiceUUID = "00030000-0001-1000-8000-00805F9B34FB"
	// WriteForwardCharUUID represents UUID for ble characteristic which handles forwarding of writes
	WriteForwardCharUUID = "00030000-0003-1000-8000-00805F9B34FB"
	// ReadForwardCharUUID represents UUID for ble characteristic which handles forwarding of reads
	ReadForwardCharUUID = "00030000-0004-1000-8000-00805F9B34FB"
	// ReadRssiMapCharUUID represents UUID for ble characteristic which handles forwarding of reads
	ReadRssiMapCharUUID         = "00030000-0005-1000-8000-00805F9B34FB"
	shortestPathRefreshInterval = time.Second * 5
	scanInterval                = time.Second * 2
	maxConnectAttempts          = 5
)

// BLEForwarder is a struct used to handle mesh network behaviors for forwarder
type BLEForwarder struct {
	name                    string
	addr                    string
	secret                  string
	serverAddr              string
	connectedAddr           string
	rssiMap                 models.RssiMap
	shortestPath            []string
	ctx                     context.Context
	cln                     *ble.Client
	nextHopWriteForwardChar *ble.Characteristic
	nextHopReadForwardChar  *ble.Characteristic
	serverCharacterstics    map[string]*ble.Characteristic
}

// NewBLEForwarder is a function that creates a new ble forwarder
func NewBLEForwarder(name string, addr string, secret string, serverAddr string) (*BLEForwarder, error) {
	d, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	ble.SetDefaultDevice(d)
	forwarder := &BLEForwarder{
		name, addr, secret, serverAddr, "",
		map[string]map[string]int{}, []string{},
		util.MakeINFContext(), nil, nil, nil, map[string]*ble.Characteristic{},
	}
	if err := ble.AddService(getService(forwarder)); err != nil {
		return nil, err
	}
	return forwarder, nil
}

// Run is a method that runs the forwarder forever
func (forwarder *BLEForwarder) Run() error {
	go forwarder.scanLoop()
	go forwarder.shortestPathRefresh()
	return ble.AdvertiseNameAndServices(forwarder.ctx, forwarder.name, ble.MustParse(MainServiceUUID))
}

func (forwarder *BLEForwarder) scanLoop() {
	forwarder.rssiMap[forwarder.addr] = map[string]int{}
	for {
		time.Sleep(scanInterval)
		ble.Scan(forwarder.ctx, true, func(a ble.Advertisement) {
			rssi := a.RSSI()
			addr := a.Address().String()
			forwarder.rssiMap[forwarder.addr][addr] = rssi
			// TODO: determine if addr is forwarder and read rssi map char and write result to forwarder.rssiMap
		}, nil)
	}
}

func (forwarder *BLEForwarder) shortestPathRefresh() {
	for {
		time.Sleep(shortestPathRefreshInterval)
		path, err := util.ShortestPathToServer(forwarder.addr, forwarder.serverAddr, forwarder.rssiMap)
		if err == nil {
			forwarder.shortestPath = path
			err := errors.New("")
			attempts := 0
			for err != nil && attempts < maxConnectAttempts {
				err = forwarder.connect(forwarder.shortestPath[0])
				attempts++
			}
		}
	}
}

func (forwarder *BLEForwarder) connect(hostAddr string) error {
	forwarder.connectedAddr = ""
	if forwarder.cln != nil {
		(*forwarder.cln).CancelConnection()
	}
	cln, err := ble.Connect(forwarder.ctx, func(a ble.Advertisement) bool {
		return util.AddrEqualAddr(a.Address().String(), hostAddr)
	})
	forwarder.cln = &cln
	if err != nil {
		return err
	}
	_, err = cln.ExchangeMTU(util.MTU)
	if err != nil {
		return err
	}
	p, err := cln.DiscoverProfile(true)
	if err != nil {
		return err
	}
	forwarder.connectedAddr = hostAddr
	for _, s := range p.Services {
		if forwarder.connectedAddr == forwarder.serverAddr {
			if util.UuidEqualStr(s.UUID, server.MainServiceUUID) {
				for _, c := range s.Characteristics {
					forwarder.serverCharacterstics[c.UUID.String()] = c
				}
				break
			}
		} else {
			if util.UuidEqualStr(s.UUID, MainServiceUUID) {
				for _, c := range s.Characteristics {
					if util.UuidEqualStr(c.UUID, WriteForwardCharUUID) {
						forwarder.nextHopWriteForwardChar = c
					} else if util.UuidEqualStr(c.UUID, WriteForwardCharUUID) {
						forwarder.nextHopReadForwardChar = c
					}
				}
				break
			}
		}
	}
	return nil
}

func (forwarder *BLEForwarder) isConnected() bool {
	return forwarder.connectedAddr != ""
}

func (forwarder *BLEForwarder) isConnectedToServer() bool {
	return forwarder.connectedAddr == forwarder.serverAddr
}

func newWriteForwardCharHandler(forwarder *BLEForwarder) func(req ble.Request, rsp ble.ResponseWriter) {
	return func(req ble.Request, rsp ble.ResponseWriter) {
		if !forwarder.isConnected() {
			// TODO: handle error
			return
		}
		if !forwarder.isConnectedToServer() {
			err := (*forwarder.cln).WriteCharacteristic(forwarder.nextHopWriteForwardChar, req.Data(), true)
			if err != nil {
				// TODO: handle error
				return
			}
		} else {
			// TODO: unpack data and determine correct server characteristc request
		}
	}
}

func newReadForwardCharHandler(forwarder *BLEForwarder) func(req ble.Request, rsp ble.ResponseWriter) {
	return func(req ble.Request, rsp ble.ResponseWriter) {
		if !forwarder.isConnected() {
			// TODO: handle error
			return
		}
		if !forwarder.isConnectedToServer() {
			data, err := (*forwarder.cln).ReadCharacteristic(forwarder.nextHopReadForwardChar)
			if err != nil {
				// TODO: handle error
				return
			}
			rsp.Write(data)
		} else {
			// TODO: unpack data and determine correct server characteristc request
		}
	}
}

func newReadRssiMapCharHandler(forwarder *BLEForwarder) func(req ble.Request, rsp ble.ResponseWriter) {
	pa := util.NewPacketAggregator()
	return server.GenerateReadHandler(forwarder.secret, pa, ReadRssiMapCharUUID, func(addr string, ctx context.Context) ([]byte, error) {
		return forwarder.rssiMap.Data()
	}, func(err error) {
		// TODO: handle announcement error
	})
}

func getService(forwarder *BLEForwarder) *ble.Service {
	service := ble.NewService(ble.MustParse(MainServiceUUID))

	write := ble.NewCharacteristic(ble.MustParse(WriteForwardCharUUID))
	write.HandleWrite(ble.WriteHandlerFunc(newWriteForwardCharHandler(forwarder)))
	service.AddCharacteristic(write)

	read := ble.NewCharacteristic(ble.MustParse(ReadForwardCharUUID))
	read.HandleRead(ble.ReadHandlerFunc(newReadForwardCharHandler(forwarder)))
	service.AddCharacteristic(read)

	readRssiMap := ble.NewCharacteristic(ble.MustParse(ReadRssiMapCharUUID))
	readRssiMap.HandleRead(ble.ReadHandlerFunc(newReadRssiMapCharHandler(forwarder)))
	service.AddCharacteristic(readRssiMap)

	return service
}
