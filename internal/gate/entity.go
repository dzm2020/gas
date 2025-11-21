package gate

import (
	"gas/internal/gate/codec"
	"gas/internal/gate/protocol"
	"gas/pkg/network"

	"github.com/panjf2000/gnet/v2"
)

type Entity struct {
	entity network.IEntity
	msg    *protocol.Message
	codec  codec.ICodec
}

func (s *Entity) Write(data []byte) error {
	return s.entity.Write(data)
}

func (s *Entity) Response(data []byte) error {
	msg := protocol.NewWithData(data)
	msg.Copy(s.msg)
	bin := s.codec.Encode(msg)
	return s.Write(bin)
}

// Push 主动推送消息（不需要请求的推送）
func (s *Entity) Push(cmd uint8, act uint8, data []byte) error {
	msg := protocol.NewWithData(data)
	msg.Cmd = cmd
	msg.Act = act
	bin := s.codec.Encode(msg)
	return s.Write(bin)
}

// RemoteAddr 获取远程地址
func (s *Entity) RemoteAddr() string {
	return s.entity.RemoteAddr()
}

// Close 关闭连接
func (s *Entity) Close() error {
	// 通过 gnet.Conn 关闭连接
	// network.IEntity 有 Con() 方法返回 gnet.Conn
	if conn, ok := s.entity.(interface{ Con() gnet.Conn }); ok {
		return conn.Con().Close()
	}
	return nil
}
