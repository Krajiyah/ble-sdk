package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"

	"github.com/pkg/errors"
)

// BLEReadCharacteristic is a struct representation of characteristic that can handle read operations from clients
type BLEReadCharacteristic struct {
	Uuid           string
	HandleRead     func(string, context.Context) ([]byte, error)
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

func newReadChar(server *BLEServer, uuid string, load func(string, context.Context) ([]byte, error)) *ble.Characteristic {
	c := ble.NewCharacteristic(ble.MustParse(uuid))
	c.HandleRead(ble.ReadHandlerFunc(generateReadHandler(server, uuid, load)))
	return c
}

func generateWriteHandler(server *BLEServer, uuid string, onWrite func(addr string, data []byte, err error)) func(req ble.Request, rsp ble.ResponseWriter) {
	return func(req ble.Request, rsp ble.ResponseWriter) {
		addr := getAddrFromReq(req)
		data := req.Data()
		payload, err := server.buffer.Set(data)
		if err != nil {
			onWrite(addr, nil, err)
			return
		}
		if payload == nil {
			return
		}
		onWrite(addr, payload, nil)
	}
}

func getSessionKey(uuid string, addr string) string {
	return fmt.Sprintf("session || %s || %s", uuid, addr)
}

type loadFn func(string, context.Context) ([]byte, error)

func loadIntoBuffer(buffer *util.PacketBuffer, req ble.Request, uuid string, secret string, load loadFn) (string, error) {
	addr := getAddrFromReq(req)
	session := getSessionKey(uuid, addr)
	ctx := req.Conn().Context()
	data, err := load(addr, ctx)
	if err != nil {
		return "", err
	}
	if data == nil || len(data) == 0 {
		return "", errors.New("empty data returned from read char loader")
	}
	var packets [][]byte
	packets, guid, err := util.EncodeDataAsPackets(data, secret)
	if err != nil {
		return "", err
	}
	err = buffer.SetAll(packets)
	if err != nil {
		return "", err
	}
	ctx = context.WithValue(ctx, session, guid)
	req.Conn().SetContext(ctx)
	return guid, nil
}

func generateReadHandler(server *BLEServer, uuid string, load loadFn) func(req ble.Request, rsp ble.ResponseWriter) {
	buffer := util.NewPacketBuffer(server.secret)
	return func(req ble.Request, rsp ble.ResponseWriter) {
		addr := getAddrFromReq(req)
		session := getSessionKey(uuid, addr)
		ctx := req.Conn().Context()
		var guid string
		if ctx.Value(session) != nil {
			guid = ctx.Value(session).(string)
		} else {
			var err error
			guid, err = loadIntoBuffer(buffer, req, uuid, server.secret, load)
			if err != nil {
				server.listener.OnInternalError(errors.Wrap(err, "loadIntoBuffer issue: "))
				return
			}
			ctx = req.Conn().Context()
		}
		packet, last, err := buffer.Pop(guid)
		if err != nil {
			server.listener.OnInternalError(errors.Wrap(err, "buffer.Pop issue: "))
			return
		}
		if last {
			ctx = context.WithValue(ctx, session, nil)
			req.Conn().SetContext(ctx)
		}
		rsp.Write(packet)
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
