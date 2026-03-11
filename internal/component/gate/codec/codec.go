// Package codec 实现网关协议的二进制编解码：大端序 13 字节头 + 变长 Body，单条消息最大 1MB。
package codec

import (
	"encoding/binary"
	"errors"

	"github.com/dzm2020/gas/internal/component/gate/protocol"
)

// MaxMsgSize 单条消息 body 最大长度（1MB），Encode/Decode 超过此长度会返回错误；可供上层配置或校验复用。
const MaxMsgSize = 1024 * 1024

const maxMsgSize = MaxMsgSize

// Encode
//
//	@Description: 将 Message 编码为字节流，超长返回错误。
//	@param msg
//	@return []byte
//	@return error
func Encode(msg *protocol.Message) ([]byte, error) {
	if msg == nil {
		return nil, errors.New("protocol encode msg is nil")
	}
	dataLen := uint32(len(msg.Data))
	if dataLen >= maxMsgSize {
		return nil, errors.New("message too large")
	}
	buf := make([]byte, protocol.HeadLen+len(msg.Data))
	offset := 0
	binary.BigEndian.PutUint32(buf[offset:], dataLen)
	offset += 4
	buf[offset] = msg.GetCmd()
	offset += 1
	buf[offset] = msg.GetAct()
	offset += 1
	binary.BigEndian.PutUint16(buf[offset:], msg.GetError())
	offset += 2
	binary.BigEndian.PutUint32(buf[offset:], msg.GetIndex())
	offset += 4
	buf[offset] = msg.GetTag()
	offset += 1
	copy(buf[offset:], msg.Data)
	return buf, nil
}

// Decode
//
//	@Description: 从 buf 解出一个完整包，返回消息与消费字节数。
//	@param buf
//	@return *protocol.Message
//	@return int
//	@return error
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

	offset := 0
	bodyLen := binary.BigEndian.Uint32(buf[offset : offset+4])
	offset += 4
	cmd := buf[offset]
	offset += 1
	act := buf[offset]
	offset += 1
	errCode := binary.BigEndian.Uint16(buf[offset : offset+2])
	offset += 2
	index := binary.BigEndian.Uint32(buf[offset : offset+4])
	offset += 4
	tag := buf[offset]
	offset += 1
	data := make([]byte, bodyLen)
	copy(data, buf[offset:])
	msg := protocol.NewDecoded(bodyLen, cmd, act, errCode, index, tag, data)

	return msg, total, nil
}
