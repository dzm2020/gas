package network

import (
	"github.com/panjf2000/gnet/v2"
)

func newTcpEntity(opts *Options, rawCon gnet.Conn) IEntity {
	entity := &baseEntity{
		id:   autoId.Add(1),
		Conn: rawCon,
		opts: opts,
	}
	entity.init()
	return &tcpEntity{entity}
}

type tcpEntity struct {
	*baseEntity
}

func (m *tcpEntity) Write(data []byte) error {
	return m.Con().AsyncWrite(data, nil)
}
