package util

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/gob"
	"errors"

	"github.com/bradfitz/slice"
	mapset "github.com/deckarep/golang-set"
	"github.com/google/uuid"
)

const (
	errCheckSumMismatch     = "Packet data is corrupted! Reason: checksum mismatch"
	errTotalMismatch        = "Packet data is corrupted! Reason: packet total mismatch"
	errIndexOutOfBounds     = "Packet data is corrupted! Reason: packet index out of bounds"
	errMixedPackets         = "Packet data is corrupted! Reason: packets are from mixed packet streams"
	errNotEnoughPackets     = "Packet data is corrupted! Reason: not enough packets were received"
	errNoPacketDataInStream = "Can't pop packet data since no packets in stream from input guid"
)

// BLEPacket is a struct used for packaging data within MTU chunks
type BLEPacket struct {
	RawData  []byte
	Guid     string
	Index    int
	Total    int
	Checksum string
}

// PacketAggregator is a struct with function to get constructed & validated data
type PacketAggregator struct {
	store map[string][]BLEPacket
}

// Data will serialize BLEPacket to []byte
func (p *BLEPacket) Data() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(p)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// NewPacketAggregator makes new struct for packet aggregation
func NewPacketAggregator() PacketAggregator {
	return PacketAggregator{map[string][]BLEPacket{}}
}

func getPacketsFromData(data []byte) []BLEPacket {
	l := []BLEPacket{}
	guid := getUUID()
	checksum := GetChecksum(data)
	chunks := splitBytes(data, MTU)
	total := len(chunks)
	for i := 0; i < total; i++ {
		p := BLEPacket{}
		p.RawData = chunks[i]
		p.Guid = guid
		p.Index = i
		p.Total = total
		p.Checksum = checksum
		l = append(l, p)
	}
	return l
}

// PopAllDataFromPackets aggregates all the packets and confirms if data accurate
func (pa *PacketAggregator) PopAllDataFromPackets(guid string) ([]byte, error) {
	ret, err := pa.getAllDataFromPackets(guid)
	if err != nil {
		return nil, err
	}
	pa.store[guid] = []BLEPacket{}
	return ret, nil
}

// AddPacketFromPacketBytes will take the input bytes, parse to packet struct, and add to store (returns guid)
func (pa *PacketAggregator) AddPacketFromPacketBytes(packetBytes []byte) (string, error) {
	buf := bytes.NewBuffer(packetBytes)
	dec := gob.NewDecoder(buf)
	var packet BLEPacket
	err := dec.Decode(&packet)
	if err != nil {
		return "", err
	}
	pa.initStore(packet.Guid)
	tmp := pa.store[packet.Guid]
	tmp = append(tmp, packet)
	pa.store[packet.Guid] = tmp
	return packet.Guid, nil
}

// AddData will take input bytes, get packets from bytes, and call AddPacketFromPacketBytes black box
func (pa *PacketAggregator) AddData(data []byte) (string, error) {
	packets := getPacketsFromData(data)
	var guid string
	for _, packet := range packets {
		data, err := packet.Data()
		if err != nil {
			return "", err
		}
		guid, err = pa.AddPacketFromPacketBytes(data)
		if err != nil {
			return "", err
		}
	}
	return guid, nil
}

// HasDataFromPacketStream returns true if all packet data is in correctly for a given guid stream
func (pa *PacketAggregator) HasDataFromPacketStream(guid string) bool {
	_, err := pa.getAllDataFromPackets(guid)
	return err == nil
}

// PopPacketDataFromStream returns next packet in (index) order in the given guid stream
func (pa *PacketAggregator) PopPacketDataFromStream(guid string) ([]byte, bool, error) {
	pa.initStore(guid)
	if len(pa.store[guid]) == 0 {
		return nil, false, errors.New(errNoPacketDataInStream)
	}
	packet := pa.store[guid][0]
	data, err := packet.Data()
	if err != nil {
		return nil, false, err
	}
	tmp := pa.store[guid]
	tmp = tmp[1:]
	pa.store[guid] = tmp
	isLastPacket := packet.Index+1 == packet.Total
	return data, isLastPacket, nil
}

func (pa *PacketAggregator) initStore(guid string) {
	if _, ok := pa.store[guid]; !ok {
		pa.store[guid] = []BLEPacket{}
	}
}

func (pa *PacketAggregator) getAllDataFromPackets(guid string) ([]byte, error) {
	pa.initStore(guid)
	packets := pa.store[guid]
	slice.Sort(packets, func(i, j int) bool {
		return packets[i].Index < packets[j].Index
	})
	ret := []byte{}
	for _, packet := range packets {
		ret = append(ret, packet.RawData...)
	}
	total := len(packets)
	checksum := GetChecksum(ret)
	guids := mapset.NewSet()
	indexes := mapset.NewSet()
	for _, packet := range packets {
		guids.Add(packet.Guid)
		indexes.Add(packet.Index)
		if checksum != packet.Checksum {
			return nil, errors.New(errCheckSumMismatch)
		}
		if total != packet.Total {
			return nil, errors.New(errTotalMismatch)
		}
		if packet.Index < 0 || packet.Index >= total {
			return nil, errors.New(errIndexOutOfBounds)
		}
	}
	if len(guids.ToSlice()) != 1 {
		return nil, errors.New(errMixedPackets)
	}
	if len(indexes.ToSlice()) != total {
		return nil, errors.New(errNotEnoughPackets)
	}
	return ret, nil
}

func getUUID() string {
	val := uuid.New()
	v := val.String()
	return v[0:8]
}

// GetChecksum is utility which allows you to easily get checksum string for given byte array
func GetChecksum(data []byte) string {
	h := sha256.New()
	h.Write(data)
	hash := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return string(hash)
}

func splitBytes(data []byte, lim int64) [][]byte {
	var chunk []byte
	chunks := make([][]byte, 0, int64(len(data))/lim+1)
	for int64(len(data)) >= lim {
		chunk, data = data[:lim], data[lim:]
		chunks = append(chunks, chunk)
	}
	if len(data) > 0 {
		chunks = append(chunks, data[:len(data)])
	}
	return chunks
}
