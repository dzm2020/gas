/**
 * @Author: dingQingHui
 * @Description:
 * @File: baseEntity
 * @Version: 1.0.0
 * @Date: 2024/12/5 15:32
 */

package network

import (
	"sync/atomic"

	"github.com/panjf2000/gnet/v2"
)

var autoId atomic.Uint64

func newEntity(opts *Options, rawCon gnet.Conn) IEntity {
	entity := &baseEntity{
		id:   autoId.Add(1),
		Conn: rawCon,
		opts: opts,
	}
	entity.init()
	return entity
}

type baseEntity struct {
	id uint64
	gnet.Conn
	opts              *Options
	network           string
	localAddr         string
	remoteAddr        string
	lastHeartBeatTime int64
	session           ISession
}

func (m *baseEntity) init() {
	if m.Conn != nil && m.Conn.RemoteAddr() != nil {
		m.network = m.Conn.RemoteAddr().Network()
		m.remoteAddr = m.Conn.RemoteAddr().String()
	}
	if m.Conn != nil && m.Conn.LocalAddr() != nil {
		m.localAddr = m.Conn.LocalAddr().String()
	}
}

func (m *baseEntity) ID() uint64 {
	return m.id
}
func (m *baseEntity) Network() string {
	return m.network
}
func (m *baseEntity) LocalAddr() string {
	return m.localAddr
}
func (m *baseEntity) RemoteAddr() string {
	return m.remoteAddr
}
func (m *baseEntity) Con() gnet.Conn {
	return m.Conn
}

func (m *baseEntity) Write(data []byte) error {
	//TODO implement me
	panic("implement me")
}

func (m *baseEntity) OnOpen(entity IEntity) (out []byte, action gnet.Action) {
	m.session = m.opts.sessionFactory()
	if err := m.session.OnConnect(entity); err != nil {
		return nil, gnet.None
	}
	return
}

func (m *baseEntity) OnTraffic(entity IEntity) (action gnet.Action) {
	//msgList := protocol.Decode(m.rawCon)
	//for _, msg := range msgList {
	//	if err := m.process(entity, msg); err != nil {
	//		return gnet.Close
	//	}
	//}
	if err := m.session.OnTraffic(); err != nil {
		return gnet.Close
	}
	return gnet.None
}

func (m *baseEntity) OnClose(entity IEntity, err error) (action gnet.Action) {
	if err := m.session.OnClose(err); err != nil {
		return gnet.Close
	}
	return gnet.None
}
