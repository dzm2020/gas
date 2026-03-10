package lib

import (
	"encoding/json"
	"errors"

	"github.com/vmihailenco/msgpack/v5"
	"google.golang.org/protobuf/proto"
)

var (
	ErrNotPBMsg = errors.New("不是pb消息")
)

var (
	Json    = new(jsonCodec)
	MsgPack = new(msgPackCodec)
	PB      = new(pbCodec)
)

type ISerializer interface {
	Unmarshal(data []byte, msg interface{}) error
	Marshal(msg interface{}) ([]byte, error)
}

// unmarshalPreCheck 统一 Unmarshal 前置检查，若已处理则返回 (true, 应返回的 error)。
func unmarshalPreCheck(data []byte, msg interface{}) (handled bool, retErr error) {
	if len(data) == 0 || msg == nil {
		return true, nil
	}
	if ptr, ok := msg.(*[]byte); ok {
		*ptr = data
		return true, nil
	}
	return false, nil
}

// marshalPreCheck 统一 Marshal 前置检查，若已处理则返回 (结果字节, true)。
func marshalPreCheck(msg interface{}) (data []byte, handled bool) {
	if msg == nil {
		return []byte{}, true
	}
	if b, ok := msg.([]byte); ok {
		return b, true
	}
	return nil, false
}

// note:Go 的 encoding/json 会把非法 UTF-8 字节替换为 U+FFFD（UTF-8 为 0xEF 0xBF 0xBD）
type jsonCodec struct{}

func (p *jsonCodec) Unmarshal(data []byte, msg interface{}) error {
	if ok, err := unmarshalPreCheck(data, msg); ok {
		return err
	}
	return json.Unmarshal(data, msg)
}

func (p *jsonCodec) Marshal(msg interface{}) ([]byte, error) {
	if data, ok := marshalPreCheck(msg); ok {
		return data, nil
	}
	return json.Marshal(msg)
}

type msgPackCodec struct{}

func (p *msgPackCodec) Unmarshal(data []byte, msg interface{}) error {
	if ok, err := unmarshalPreCheck(data, msg); ok {
		return err
	}
	return msgpack.Unmarshal(data, msg)
}

func (p *msgPackCodec) Marshal(msg interface{}) ([]byte, error) {
	if data, ok := marshalPreCheck(msg); ok {
		return data, nil
	}
	return msgpack.Marshal(msg)
}

type pbCodec struct{}

func (p *pbCodec) Unmarshal(data []byte, msg interface{}) error {
	if ok, err := unmarshalPreCheck(data, msg); ok {
		return err
	}
	v, ok := msg.(proto.Message)
	if !ok {
		return ErrNotPBMsg
	}
	return proto.Unmarshal(data, v)
}

func (p *pbCodec) Marshal(msg interface{}) ([]byte, error) {
	if data, ok := marshalPreCheck(msg); ok {
		return data, nil
	}
	v, ok := msg.(proto.Message)
	if !ok {
		return nil, ErrNotPBMsg
	}
	return proto.Marshal(v)
}
