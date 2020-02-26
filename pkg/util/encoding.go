package util

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
)

func decompress(data []byte) (resData []byte, err error) {
	b := bytes.NewBuffer(data)
	var r io.Reader
	r, err = gzip.NewReader(b)
	if err != nil {
		return
	}
	var resB bytes.Buffer
	_, err = resB.ReadFrom(r)
	if err != nil {
		return
	}
	resData = resB.Bytes()
	return
}

func compress(data []byte) (compressedData []byte, err error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, err = gz.Write(data)
	if err != nil {
		return
	}
	if err = gz.Flush(); err != nil {
		return
	}
	if err = gz.Close(); err != nil {
		return
	}
	compressedData = b.Bytes()
	return
}

func Encode(x interface{}) ([]byte, error) {
	data, err := json.Marshal(x)
	if err != nil {
		return nil, err
	}
	return compress(data)
}

func Decode(data []byte, x interface{}) error {
	data, err := decompress(data)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, x)
}
