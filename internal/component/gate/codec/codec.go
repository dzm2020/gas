// Package codec 实现网关协议的二进制编解码：大端序 12 字节头 + 变长 Body，单条消息最大 1MB。
package codec

import (
	"encoding/binary"
	"errors"

	"github.com/dzm2020/gas/internal/component/gate/protocol"
)

const (
	maxMsgSize = 1024 * 1024 // 单条消息最大 1MB，超过则 Encode/Decode 报错
)

// Encode 将 protocol.Message 编码为字节流：HeadLen 字节头（Len 按 len(Data) 写入）+ Data；消息体超过 maxMsgSize 返回错误。
func Encode(msg *protocol.Message) ([]byte, error) {
	dataLen := uint32(len(msg.Data))
	if dataLen >= maxMsgSize {
		return nil, errors.New("message too large")
	}
	buf := make([]byte, protocol.HeadLen+len(msg.Data))
	offset := 0
	binary.BigEndian.PutUint32(buf[offset:], dataLen)
	offset += 4
	buf[offset] = msg.Cmd
	offset += 1
	buf[offset] = msg.Act
	offset += 1
	binary.BigEndian.PutUint16(buf[offset:], msg.Error)
	offset += 2
	binary.BigEndian.PutUint32(buf[offset:], msg.Index)
	offset += 4
	copy(buf[offset:], msg.Data)
	return buf, nil
}

// Decode 从 buf 解出一个完整包：若数据不足或 Len 非法返回 (nil, 0, nil/nil)；成功返回 (*Message, 消费字节数, nil)。
func Decode(buf []byte) (*protocol.Message, int, error) {
	if len(buf) < protocol.HeadLen {
		return nil, 0, nil
	}
	l := binary.BigEndian.Uint32(buf)
	if l >= maxMsgSize {
		return nil, 0, errors.New("message too large")
	}
	total := protocol.HeadLen + int(l)
	if len(buf) < total {
		return nil, 0, nil
	}

	msg := &protocol.Message{
		Head: &protocol.Head{},
	}
	offset := 0
	msg.Len = binary.BigEndian.Uint32(buf[offset : offset+4])
	offset += 4
	msg.Cmd = buf[offset]
	offset += 1
	msg.Act = buf[offset]
	offset += 1
	msg.Error = binary.BigEndian.Uint16(buf[offset : offset+2])
	offset += 2
	msg.Index = binary.BigEndian.Uint32(buf[offset : offset+4])
	offset += 4
	msg.Data = make([]byte, msg.Len)
	copy(msg.Data, buf[offset:])

	return msg, total, nil
}
