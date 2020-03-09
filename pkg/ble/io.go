package ble

import (
	"fmt"
	"sync"

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

func (c *RealConnection) doWrite(wg *sync.WaitGroup, char *ble.Characteristic, packet []byte) {
	retryAndOptimizeReadOrWrite(c, "WriteCharacteristic", func() error {
		return c.cln.WriteCharacteristic(char, packet, true)
	})
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
