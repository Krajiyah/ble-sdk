package ble

import (
	"fmt"

	"github.com/Krajiyah/ble-sdk/pkg/util"
	"github.com/go-ble/ble"
	"github.com/pkg/errors"
)

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
	retryAndOptimizeReadOrWrite(c, "ReadLongCharacteristic", func() error {
		dat, e := c.cln.ReadLongCharacteristic(char)
		if e != nil {
			fmt.Println("Read Issue: " + e.Error())
			return e
		}
		go func() { encDataBuff <- dat }()
		return nil
	})
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

func (c *RealConnection) doWrite(uuid string, data []byte, blocking bool) error {
	char, packets, err := c.prepWriteValue(uuid, data)
	if err != nil {
		return err
	}
	for _, packet := range packets {
		retryAndOptimizeReadOrWrite(c, "BlockingWriteValue (packet)", func() error {
			return c.cln.WriteCharacteristic(char, packet, !blocking)
		})
	}
	return nil
}

func (c *RealConnection) BlockingWriteValue(uuid string, data []byte) error {
	return c.doWrite(uuid, data, true)
}

func (c *RealConnection) NonBlockingWriteValue(uuid string, data []byte) {
	c.doWrite(uuid, data, false)
}
