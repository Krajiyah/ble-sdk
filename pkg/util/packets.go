package util

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"

	"github.com/bradfitz/slice"
)

const (
	guidSize   = 16
	headerSize = guidSize + 4 + 4 + 4
)

type header struct {
	Guid        []byte
	Index       uint32
	Total       uint32
	PayloadSize uint32
}

func numToBytes(x uint32) []byte {
	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, x)
	return bs
}

func bytesToNum(b []byte) uint32 {
	return binary.LittleEndian.Uint32(b)
}

func getRandBytes(size int) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}

func split(buf []byte, lim int) [][]byte {
	var chunk []byte
	chunks := make([][]byte, 0, len(buf)/lim+1)
	for len(buf) >= lim {
		chunk, buf = buf[:lim], buf[lim:]
		chunks = append(chunks, chunk)
	}
	if len(buf) > 0 {
		chunks = append(chunks, buf[:len(buf)])
	}
	return chunks
}

func encodeToPacket(chunk []byte, h header) ([]byte, error) {
	packet := bytes.NewBuffer([]byte{})
	packet.Write([]byte(h.Guid))
	packet.Write(numToBytes(h.Index))
	packet.Write(numToBytes(h.Total))
	packet.Write(numToBytes(uint32(len(chunk))))
	packet.Write(chunk)
	b := packet.Bytes()
	if len(b) > MTU {
		return nil, errors.New("MTU overflow")
	}
	return b, nil
}

func decodeFromPacket(data []byte) (*header, []byte, error) {
	packet := bytes.NewBuffer(data)
	guid := make([]byte, guidSize)
	_, err := packet.Read(guid)
	if err != nil {
		return nil, nil, err
	}
	index := make([]byte, 4)
	_, err = packet.Read(index)
	if err != nil {
		return nil, nil, err
	}
	total := make([]byte, 4)
	_, err = packet.Read(total)
	if err != nil {
		return nil, nil, err
	}
	payloadSize := make([]byte, 4)
	_, err = packet.Read(payloadSize)
	if err != nil {
		return nil, nil, err
	}
	payload := make([]byte, bytesToNum(payloadSize))
	_, err = packet.Read(payload)
	if err != nil {
		return nil, nil, err
	}
	header := &header{
		Guid:        guid,
		Index:       bytesToNum(index),
		Total:       bytesToNum(total),
		PayloadSize: bytesToNum(payloadSize),
	}
	return header, payload, nil
}

func EncodeDataAsPackets(data []byte) ([][]byte, error) {
	guid := getRandBytes(guidSize)
	dataLength := uint32(len(data))
	chunks := split(data, MTU-headerSize)
	total := len(chunks)
	packets := [][]byte{}
	for i, chunk := range chunks {
		h := header{
			Guid:        guid,
			Index:       uint32(i),
			Total:       uint32(total),
			PayloadSize: dataLength,
		}
		packet, err := encodeToPacket(chunk, h)
		if err != nil {
			return nil, err
		}
		packets = append(packets, packet)
	}
	return packets, nil
}

type packetSortable struct {
	index uint32
	data  []byte
}

func DecodePacketsToData(packets [][]byte) ([]byte, error) {
	sortables := []packetSortable{}
	for _, packet := range packets {
		h, chunk, err := decodeFromPacket(packet)
		if err != nil {
			return nil, err
		}
		sortables = append(sortables, packetSortable{index: h.Index, data: chunk})
	}
	slice.Sort(sortables, func(i, j int) bool {
		return sortables[i].index < sortables[j].index
	})
	data := []byte{}
	for _, b := range sortables {
		data = append(data, b.data...)
	}
	return data, nil
}

func GetHeaderFromPacket(packet []byte) (*header, error) {
	h, _, err := decodeFromPacket(packet)
	if err != nil {
		return nil, err
	}
	return h, nil
}
