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
	if data == nil || msg == nil {
		return ErrJsonUnPack
	}
	err := json.Unmarshal(data, msg)
	if err != nil {
		return err
	}
	return nil
}

func (p *jsonCodec) Marshal(msg interface{}) ([]byte, error) {
	if msg == nil {
		return nil, ErrJsonPack
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	return data, nil
}

type msgPackCodec struct {
}

func (p *msgPackCodec) Unmarshal(data []byte, msg interface{}) error {
	err := msgpack.Unmarshal(data, msg)
	return err
}

func (p *msgPackCodec) Marshal(msg interface{}) ([]byte, error) {
	data, err := msgpack.Marshal(msg)
	return data, err
}

type pbCodec struct {
}

func (p *pbCodec) Unmarshal(data []byte, msg interface{}) error {
	if msg == nil {
		return ErrPBUnPack
	}
	v, ok := msg.(proto.Message)
	if !ok {
		return ErrNotPBMsg
	}
	err := proto.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}

func (p *pbCodec) Marshal(msg interface{}) ([]byte, error) {
	if msg == nil {
		return nil, ErrPBPack
	}
	v, ok := msg.(proto.Message)
	if !ok {
		return nil, ErrNotPBMsg
	}
	data, err := proto.Marshal(v)
	if err != nil {
		return nil, err
	}

	return data, nil
}
