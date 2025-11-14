package network

import (
	"sync"

	"github.com/panjf2000/gnet/v2"
)

func newUdpServer(opts *Options, protoAddr string) *udpServer {
	m := &udpServer{
		baseServer: newBaseServer(opts, protoAddr),
		dict:       make(map[string]IEntity),
	}
	return m
}

type udpServer struct {
	*baseServer
	dict map[string]IEntity
	sync.RWMutex
}

func (m *udpServer) Serve() error {
	return gnet.Run(m, m.protoAddr, m.Options().opts...)
}

func (m *udpServer) OnTraffic(c gnet.Conn) (action gnet.Action) {
	remoteAddr := c.RemoteAddr().String()

	m.RLock()
	entity, ok := m.dict[remoteAddr]
	m.RUnlock()
	if ok {
		return entity.OnTraffic(entity)
	}

	m.Lock()
	defer m.Unlock()
	entity, ok = m.dict[remoteAddr]
	if ok {
		return entity.OnTraffic(entity)
	}

	entity = newUdpEntity(m.opts, c)
	entity.OnOpen(entity)
	m.dict[remoteAddr] = entity
	return entity.OnTraffic(entity)
}
