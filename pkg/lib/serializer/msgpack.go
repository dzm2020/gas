/**
 * @Author: dingQingHui
 * @Description:
 * @File: msgpack
 * @Version: 1.0.0
 * @Date: 2024/11/19 18:20
 */

package serializer

import "github.com/vmihailenco/msgpack/v5"

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
