/**
 * @Author: dingQingHui
 * @Description:
 * @File: json
 * @Version: 1.0.0
 * @Date: 2024/11/19 18:19
 */

package serializer

import (
	"encoding/json"
)

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
