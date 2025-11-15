package codec

import (
	"encoding/binary"
	"gas/internal/gate/protocol"
)

// ICodec 编解码器接口
type ICodec interface {
	Encode(msg *protocol.Message) []byte                        // 编码（消息→字节流）
	Decode(buf []byte) (msgList []*protocol.Message, readN int) // 解码（字节流→消息）
}

func New() ICodec {
	return new(Codec)
}

type Codec struct {
}

func (*Codec) Encode(msg *protocol.Message) []byte {
	msg.Len = uint32(len(msg.Data))
	buf := make([]byte, protocol.HeadLen+len(msg.Data))
	offset := 0
	binary.BigEndian.PutUint32(buf[offset:], msg.Len)
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
	return buf
}

func (*Codec) Decode(buf []byte) (msgList []*protocol.Message, readN int) {

	for {
		if len(buf) < protocol.HeadLen {
			return
		}
		l := binary.BigEndian.Uint32(buf)
		total := protocol.HeadLen + int(l)
		if len(buf) < total {
			return
		}

		msg := protocol.New()
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

		msgList = append(msgList, msg)

		buf = buf[total:]

		readN += total
	}
}
