package network

import (
	"github.com/panjf2000/gnet/v2"
)

func newTcpServer(opts *Options, protoAddr string) *tcpServer {
	m := &tcpServer{
		baseServer: newBaseServer(opts, protoAddr),
	}
	return m
}

type tcpServer struct {
	*baseServer
}

func (m *tcpServer) Serve() error {
	return gnet.Run(m, m.protoAddr, m.opts.opts...)
}

func (m *tcpServer) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	entity := newTcpEntity(m.opts, c)
	c.SetContext(entity)
	entity.OnOpen(entity)
	return nil, gnet.None
}

func (m *tcpServer) OnTraffic(c gnet.Conn) (action gnet.Action) {
	entity := c.Context().(IEntity)
	if entity == nil {
		return gnet.Close
	}
	return entity.OnTraffic(entity)
}

func (m *tcpServer) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	entity := c.Context().(IEntity)
	if entity == nil {
		return gnet.Close
	}
	return entity.OnClose(entity, err)
}
