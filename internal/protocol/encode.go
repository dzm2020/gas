package protocol

import (
	"encoding/binary"

	"github.com/panjf2000/gnet/v2"
)

// ICodec 编解码器接口
type ICodec interface {
	Encode(msg *Message) []byte                     // 编码（消息→字节流）
	Decode(reader gnet.Reader) (msgList []*Message) // 解码（字节流→消息）
}

type Codec struct {
}

func (*Codec) Encode(msg *Message) []byte {
	msg.Len = uint32(len(msg.Data))
	buf := make([]byte, HeadLen+len(msg.Data))
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

func (*Codec) Decode(reader gnet.Reader) (msgList []*Message, err error) {
	for {
		var buf []byte
		if reader.InboundBuffered() < HeadLen {
			return
		}
		buf, err = reader.Peek(HeadLen)
		if err != nil {
			return
		}
		l := binary.BigEndian.Uint32(buf)
		total := HeadLen + int(l)
		if reader.InboundBuffered() < total {
			return
		}

		buf, err = reader.Peek(total)
		if err != nil {
			return
		}

		msg := New()
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

		_, _ = reader.Discard(total)
	}
}
