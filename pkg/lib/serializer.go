package lib

import (
	"encoding/json"
	"errors"

	"github.com/vmihailenco/msgpack/v5"
	"google.golang.org/protobuf/proto"
)

var (
	ErrMsgPackPack   = errors.New("msgpack打包错误")
	ErrMsgPackUnPack = errors.New("msgpack解析错误")
	ErrPBPack        = errors.New("pb打包错误")
	ErrPBUnPack      = errors.New("pb解析错误")
	ErrNotPBMsg      = errors.New("不是pb消息")
	ErrJsonPack      = errors.New("json打包错误")
	ErrJsonUnPack    = errors.New("json解析错误")
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

type jsonCodec struct {
}

func (p *jsonCodec) Unmarshal(data []byte, msg interface{}) error {
	if len(data) == 0 {
		return nil
	}
	if msg == nil {
		return nil
	}
	if ptr, ok := msg.(*[]byte); ok {
		*ptr = data
		return nil
	}
	return json.Unmarshal(data, msg)
}

func (p *jsonCodec) Marshal(msg interface{}) ([]byte, error) {
	if msg == nil {
		return []byte{}, nil
	}
	if data, ok := msg.([]byte); ok {
		return data, nil
	}
	return json.Marshal(msg)
}

type msgPackCodec struct {
}

func (p *msgPackCodec) Unmarshal(data []byte, msg interface{}) error {
	if len(data) == 0 {
		return nil
	}
	if msg == nil {
		return nil
	}
	if ptr, ok := msg.(*[]byte); ok {
		*ptr = data
		return nil
	}
	return msgpack.Unmarshal(data, msg)
}

func (p *msgPackCodec) Marshal(msg interface{}) ([]byte, error) {
	if msg == nil {
		return []byte{}, nil
	}
	if data, ok := msg.([]byte); ok {
		return data, nil
	}
	return msgpack.Marshal(msg)
}

type pbCodec struct {
}

func (p *pbCodec) Unmarshal(data []byte, msg interface{}) error {
	if len(data) == 0 {
		return nil
	}
	if msg == nil {
		return nil
	}
	if ptr, ok := msg.(*[]byte); ok {
		*ptr = data
		return nil
	}
	v, ok := msg.(proto.Message)
	if !ok {
		return ErrNotPBMsg
	}
	return proto.Unmarshal(data, v)
}

func (p *pbCodec) Marshal(msg interface{}) ([]byte, error) {
	if msg == nil {
		return []byte{}, nil
	}
	if data, ok := msg.([]byte); ok {
		return data, nil
	}
	v, ok := msg.(proto.Message)
	if !ok {
		return nil, ErrNotPBMsg
	}
	return proto.Marshal(v)
}
