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

type IEntity interface {
	ID() uint64
	Write(data []byte) error
	Network() string
	LocalAddr() string
	RemoteAddr() string
	Read(p []byte) (n int, err error)
	SetContext(interface{})
	Context() interface{}
	Close() error
}

var autoId atomic.Uint64

func newEntity(opts *Options, rawCon gnet.Conn) *baseEntity {
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
	opts       *Options
	network    string
	localAddr  string
	remoteAddr string
	ctx        interface{}
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

func (m *baseEntity) Read(p []byte) (n int, err error) {
	n = min(m.InboundBuffered(), len(p))
	return m.Con().Read(p[:n])
}

func (m *baseEntity) SetContext(ctx interface{}) {
	m.ctx = ctx
}
func (m *baseEntity) Context() interface{} {
	return m.ctx
}

func (m *baseEntity) Close() error {
	return m.Con().Close()
}

// newTcpEntity
//
//	@Description: tcp
//	@param opts
//	@param rawCon
//	@return IEntity
func newTcpEntity(opts *Options, rawCon gnet.Conn) IEntity {
	entity := newEntity(opts, rawCon)
	entity.init()
	return &tcpEntity{entity}
}

type tcpEntity struct {
	*baseEntity
}

func (m *tcpEntity) Write(data []byte) error {
	return m.Con().AsyncWrite(data, nil)
}

// newUdpEntity
//
//	@Description: udp
//	@param opts
//	@param rawCon
//	@return IEntity
func newUdpEntity(opts *Options, rawCon gnet.Conn) IEntity {
	entity := newEntity(opts, rawCon)
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
