package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/currantlabs/ble"
)

// BLEReadCharacteristic is a struct representation of characteristic that can handle read operations from clients
type BLEReadCharacteristic struct {
	Uuid           string
	HandleRead     func(context.Context) ([]byte, error)
	DoInBackground func()
}

// BLEWriteCharacteristic is a struct representation of characteristic that can handle write  operations from clients
type BLEWriteCharacteristic struct {
	Uuid           string
	HandleWrite    func(addr string, data []byte, err error)
	DoInBackground func()
}

func getAddrFromReq(req ble.Request) string {
	return strings.ToUpper(req.Conn().RemoteAddr().String())
}

func newWriteChar(server *BLEServer, uuid string, onWrite func(addr string, data []byte, err error)) *ble.Characteristic {
	c := ble.NewCharacteristic(ble.MustParse(uuid))
	c.HandleWrite(ble.WriteHandlerFunc(generateWriteHandler(server, uuid, onWrite)))
	return c
}

func newReadChar(server *BLEServer, uuid string, load func(context.Context) ([]byte, error)) *ble.Characteristic {
	c := ble.NewCharacteristic(ble.MustParse(uuid))
	c.HandleRead(ble.ReadHandlerFunc(generateReadHandler(server, uuid, load)))
	return c
}

func generateWriteHandler(server *BLEServer, uuid string, onWrite func(addr string, data []byte, err error)) func(req ble.Request, rsp ble.ResponseWriter) {
	return func(req ble.Request, rsp ble.ResponseWriter) {
		addr := getAddrFromReq(req)
		guid, err := server.packetAggregator.AddPacketFromPacketBytes(req.Data())
		if err != nil {
			onWrite(addr, nil, err)
			return
		}
		if server.packetAggregator.HasDataFromPacketStream(guid) {
			encryptedData, _ := server.packetAggregator.PopAllDataFromPackets(guid)
			data, err := util.Decrypt(encryptedData, server.secret)
			if err != nil {
				onWrite(addr, nil, err)
				return
			}
			onWrite(addr, data, nil)
		}
	}
}

type sessionKeyType string

func getSessionKey(uuid string, addr string) sessionKeyType {
	return sessionKeyType(fmt.Sprintf("session || %s || %s", uuid, addr))
}

func generateReadHandler(server *BLEServer, uuid string, load func(context.Context) ([]byte, error)) func(req ble.Request, rsp ble.ResponseWriter) {
	return func(req ble.Request, rsp ble.ResponseWriter) {
		addr := getAddrFromReq(req)
		sessionKey := getSessionKey(uuid, addr)
		ctx := req.Conn().Context()
		var guid string
		if ctx.Value(sessionKey) != nil {
			guid = ctx.Value(sessionKey).(string)
		} else {
			data, err := load(ctx)
			if err != nil {
				server.listener.OnReadOrWriteError(err)
				return
			}
			encryptedData, err := util.Encrypt(data, server.secret)
			if err != nil {
				server.listener.OnReadOrWriteError(err)
				return
			}
			guid, err = server.packetAggregator.AddData(encryptedData)
			if err != nil {
				server.listener.OnReadOrWriteError(err)
				return
			}
			ctx = context.WithValue(ctx, sessionKey, guid)
			req.Conn().SetContext(ctx)
		}
		packetData, isLastPacket, err := server.packetAggregator.PopPacketDataFromStream(guid)
		if isLastPacket {
			ctx = context.WithValue(ctx, sessionKey, nil)
			req.Conn().SetContext(ctx)
		}
		if err == nil {
			rsp.Write(packetData)
		}
	}
}

func constructReadChar(server *BLEServer, char *BLEReadCharacteristic) *ble.Characteristic {
	c := newReadChar(server, char.Uuid, char.HandleRead)
	go char.DoInBackground()
	return c
}

func constructWriteChar(server *BLEServer, char *BLEWriteCharacteristic) *ble.Characteristic {
	c := newWriteChar(server, char.Uuid, char.HandleWrite)
	go char.DoInBackground()
	return c
}
