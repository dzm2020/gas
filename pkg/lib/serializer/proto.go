/**
 * @Author: dingQingHui
 * @Description:
 * @File: proto
 * @Version: 1.0.0
 * @Date: 2024/11/19 18:10
 */

package serializer

import (
	"google.golang.org/protobuf/proto"
)

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
