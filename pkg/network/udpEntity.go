package network

import (
	"github.com/panjf2000/gnet/v2"
)

func newUdpEntity(opts *Options, rawCon gnet.Conn) IEntity {
	entity := &baseEntity{
		id:   autoId.Add(1),
		Conn: rawCon,
		opts: opts,
	}
	entity.init()
	return &udpEntity{entity}
}

type udpEntity struct {
	*baseEntity
}

func (m *udpEntity) Write(data []byte) error {
	_, err := m.Con().Write(data)
	return err
}
