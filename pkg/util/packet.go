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
	errCheckSumMismatch = "Packet data is corrupted! Reason: checksum mismatch"
	errTotalMismatch    = "Packet data is corrupted! Reason: packet total mismatch"
	errIndexOutOfBounds = "Packet data is corrupted! Reason: packet index out of bounds"
	errMixedPackets     = "Packet data is corrupted! Reason: packets are from mixed packet streams"
	errNotEnoughPackets = "Packet data is corrupted! Reason: not enough packets were received"
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

// GetPacketsFromData chunks data into MTU sizes
func GetPacketsFromData(data []byte) []BLEPacket {
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

// GetDataFromPackets aggregates all the packets and confirms if data accurate
func (pa *PacketAggregator) GetDataFromPackets(guid string) ([]byte, error) {
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

// AddPacketFromPacketBytes will take the input bytes, parse to packet struct, and add to store
func (pa *PacketAggregator) AddPacketFromPacketBytes(packetBytes []byte) error {
	buf := bytes.NewBuffer(packetBytes)
	dec := gob.NewDecoder(buf)
	var packet BLEPacket
	err := dec.Decode(&packet)
	if err != nil {
		return err
	}
	tmp := pa.store[packet.Guid]
	tmp = append(tmp, packet)
	pa.store[packet.Guid] = tmp
	return nil
}

// GetPacketStreams will yield list of guids which have all the packets it needs
func (pa *PacketAggregator) GetPacketStreams() []string {
	ret := []string{}
	for guid := range pa.store {
		_, err := pa.GetDataFromPackets(guid)
		if err == nil {
			ret = append(ret, guid)
		}
	}
	return ret
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
