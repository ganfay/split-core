package utils

import (
	"bytes"
	"encoding/json"
)

func DecodeJSON[T any](data []byte) (T, error) {
	var result T
	err := json.NewDecoder(bytes.NewReader(data)).Decode(&result)
	return result, err
}

func EncodeJSON[T any](data T) ([]byte, error) {
	var resp bytes.Buffer
	err := json.NewEncoder(&resp).Encode(data)
	return resp.Bytes(), err
}
