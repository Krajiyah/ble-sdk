package util

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"sync"
)

const (
	guidSize   = 32
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
	packet.Write(numToBytes(h.PayloadSize))
	packet.Write(chunk)
	b := packet.Bytes()
	if len(b) > MTU {
		return nil, errors.New("MTU overflow")
	}
	return b, nil
}

func decodeFromPacket(data []byte) (*header, []byte, error) {
	copiedData := make([]byte, len(data))
	copy(copiedData[:], data[:len(data)])
	packet := bytes.NewBuffer(copiedData)
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
	payload := make([]byte, int(bytesToNum(payloadSize)))
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

func EncodeDataAsPackets(payload []byte, secret string) ([][]byte, error) {
	fmt.Println("Original Data: " + base64.StdEncoding.EncodeToString(payload))
	data, err := Encrypt(payload, secret)
	fmt.Println("Encrypted Data: " + base64.StdEncoding.EncodeToString(data))
	if err != nil {
		return nil, err
	}
	guid := getRandBytes(guidSize)
	chunks := split(data, MTU-headerSize)
	total := len(chunks)
	packets := [][]byte{}
	for i, chunk := range chunks {
		h := header{
			Guid:        guid,
			Index:       uint32(i),
			Total:       uint32(total),
			PayloadSize: uint32(len(chunk)),
		}
		fmt.Println("GUID")
		fmt.Println(base64.StdEncoding.EncodeToString(h.Guid))
		fmt.Printf("Chunk Length: %d\n", len(chunk))
		packet, err := encodeToPacket(chunk, h)
		if err != nil {
			return nil, err
		}
		fmt.Println("Packet")
		fmt.Println(base64.StdEncoding.EncodeToString(packet))
		packets = append(packets, packet)
	}
	return packets, nil
}

type packetSortable struct {
	header *header
	chunk  []byte
}

func decodePacketsToData(packets [][]byte, secret string) ([]byte, error) {
	sortables := []packetSortable{}
	for _, packet := range packets {
		h, chunk, err := decodeFromPacket(packet)
		if err != nil {
			return nil, err
		}
		fmt.Println("GUID (v2)")
		fmt.Println(base64.StdEncoding.EncodeToString(h.Guid))
		fmt.Printf("Chunk Length (v2): %d\n", len(chunk))
		fmt.Println("Packet (v2)")
		fmt.Println(base64.StdEncoding.EncodeToString(packet))
		sortables = append(sortables, packetSortable{header: h, chunk: chunk})
	}
	sort.Slice(sortables, func(i, j int) bool {
		return sortables[i].header.Index < sortables[j].header.Index
	})
	encData := []byte{}
	for _, b := range sortables {
		encData = append(encData, b.chunk...)
	}
	fmt.Println("Encrypted Data: " + base64.StdEncoding.EncodeToString(encData))
	return Decrypt(encData, secret)
}

type PacketBuffer struct {
	mutex  *sync.Mutex
	data   map[string][][]byte
	secret string
}

func NewPacketBuffer(secret string) *PacketBuffer {
	return &PacketBuffer{secret: secret, mutex: &sync.Mutex{}, data: map[string][][]byte{}}
}

func (buff *PacketBuffer) Set(packet []byte) ([]byte, error) {
	buff.mutex.Lock()
	defer buff.mutex.Unlock()
	header, chunk, err := decodeFromPacket(packet)
	fmt.Println("GUID (v1)")
	fmt.Println(base64.StdEncoding.EncodeToString(header.Guid))
	fmt.Printf("Chunk Length (v1): %d\n", len(chunk))
	fmt.Println("Packet (v1)")
	fmt.Println(base64.StdEncoding.EncodeToString(packet))
	if err != nil {
		return nil, err
	}
	guid64 := base64.StdEncoding.EncodeToString(header.Guid)
	if _, ok := buff.data[guid64]; !ok {
		buff.data[guid64] = [][]byte{}
	}
	packets := buff.data[guid64]
	packets = append(packets, packet)
	buff.data[guid64] = packets
	if uint32(len(packets)) < header.Total {
		return nil, nil
	}
	buff.data[guid64] = [][]byte{}
	return decodePacketsToData(packets, buff.secret)
}
