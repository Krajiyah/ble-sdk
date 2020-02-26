package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Krajiyah/ble-sdk/pkg/util"
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
		payload, err := server.buffer.Set(data)
		if err != nil {
			fmt.Printf("CHAR: %s\n", uuid)
			fmt.Printf("LEN: %d\n", len(data))
			onWrite(addr, nil, err)
			return
		}
		if payload == nil {
			return
		}
		fmt.Println("WORKS!")
		onWrite(addr, payload, nil)
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
