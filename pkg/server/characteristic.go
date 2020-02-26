package server

import (
	"context"
	"errors"
	"strings"

	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/bradfitz/slice"
	"github.com/go-ble/ble"
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
		ctx := req.Conn().Context()
		guid := ctx.Value(util.WriteGuidCtxKey).(string)
		i := ctx.Value(util.WriteIndexCtxKey).(int)
		total := ctx.Value(util.WriteTotalCtxKey).(int)
		if _, ok := server.buffer[guid]; !ok {
			server.buffer[guid] = []bufferEntry{}
		}
		arr := server.buffer[guid]
		arr = append(arr, bufferEntry{Index: i, Data: data})
		server.buffer[guid] = arr
		if len(arr) < total {
			return
		}
		slice.Sort(arr, func(i, j int) bool {
			return arr[i].Index < arr[j].Index
		})
		encryptedData := []byte{}
		for _, b := range arr {
			encryptedData = append(encryptedData, b.Data...)
		}
		data, err := util.Decrypt(encryptedData, server.secret)
		if err != nil {
			onWrite(addr, nil, err)
			return
		}
		onWrite(addr, data, nil)
	}
}

func generateReadHandler(server *BLEServer, uuid string, load func(string, context.Context) ([]byte, error)) func(req ble.Request, rsp ble.ResponseWriter) {
	return func(req ble.Request, rsp ble.ResponseWriter) {
		addr := getAddrFromReq(req)
		data, err := load(addr, req.Conn().Context())
		if err != nil {
			server.listener.OnReadOrWriteError(err)
			return
		}
		if data == nil || len(data) == 0 {
			server.listener.OnReadOrWriteError(errors.New("empty data returned from read char loader"))
			return
		}
		encryptedData, err := util.Encrypt(data, server.secret)
		if err != nil {
			server.listener.OnReadOrWriteError(err)
			return
		}
		rsp.Write(encryptedData)
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
