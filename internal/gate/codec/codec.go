package codec

import (
	"encoding/binary"
	"fmt"
	"gas/internal/gate/protocol"
)

var (
	ErrInvalidCodecMessageType = fmt.Errorf("invalid message type")
)

func New() *Codec {
	return new(Codec)
}

type Codec struct {
}

func (*Codec) Encode(message interface{}) ([]byte, error) {
	msg, ok := message.(*protocol.Message)
	if !ok {
		return nil, ErrInvalidCodecMessageType
	}
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
	return buf, nil
}

func (*Codec) Decode(buf []byte) (interface{}, int, error) {
	if len(buf) < protocol.HeadLen {
		return nil, 0, nil
	}
	l := binary.BigEndian.Uint32(buf)
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
