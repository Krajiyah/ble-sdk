package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/currantlabs/ble"
)

func getAddrFromReq(req ble.Request) string {
	return strings.ToUpper(req.Conn().RemoteAddr().String())
}

func newWriteChar(server *BLEServer, uuid string, onWrite func(addr string, data []byte, err error)) *ble.Characteristic {
	c := ble.NewCharacteristic(ble.MustParse(uuid))
	c.HandleWrite(ble.WriteHandlerFunc(func(req ble.Request, rsp ble.ResponseWriter) {
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
	}))
	return c
}

func newReadChar(server *BLEServer, uuid string, load func(context.Context) ([]byte, error)) *ble.Characteristic {
	c := ble.NewCharacteristic(ble.MustParse(uuid))
	c.HandleRead(ble.ReadHandlerFunc(func(req ble.Request, rsp ble.ResponseWriter) {
		addr := getAddrFromReq(req)
		type sessionKeyType string
		var sessionKey sessionKeyType
		sessionKey = sessionKeyType(fmt.Sprintf("session || %s || %s", uuid, addr))
		ctx := req.Conn().Context()
		var guid string
		if ctx.Value(sessionKey) != nil {
			guid = ctx.Value(sessionKey).(string)
		} else {
			data, err := load(ctx)
			if err != nil {
				// TODO: how to handle error?
				return
			}
			encryptedData, err := util.Encrypt(data, server.secret)
			if err != nil {
				// TODO: how to handle error?
				return
			}
			guid, err = server.packetAggregator.AddData(encryptedData)
			if err != nil {
				// TODO: how to handle error?
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
	}))
	return c
}
