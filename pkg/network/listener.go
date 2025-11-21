/**
 * @Author: dingQingHui
 * @Description:
 * @File: server
 * @Version: 1.0.0
 * @Date: 2024/11/29 14:49
 */

package network

import (
	"context"
	"gas/pkg/utils/netx"
	"gas/pkg/utils/xerror"
	"sync"

	"github.com/panjf2000/gnet/v2"
)

type IListener interface {
	Serve() error
	Close() error // 关闭监听器
	Addr() string // 监听地址
}

func NewListener(protoAddr string, options ...Option) IListener {
	s := newListener(protoAddr, options...)
	return s
}

func newListener(protoAddr string, options ...Option) IListener {
	proto, _, err := netx.ParseProtoAddr(protoAddr)
	xerror.Assert(err)
	opts := loadOptions(options...)
	switch proto {
	case "udp", "udp4", "udp6":
		return newUdpListener(opts, protoAddr)
	case "tcp", "tcp4", "tcp6":
		return newTcpListener(opts, protoAddr)
	}
	return nil
}

func newBaseListener(opts *Options, protoAddr string) *baseListener {
	m := &baseListener{
		opts:      opts,
		protoAddr: protoAddr,
	}
	return m
}

type baseListener struct {
	gnet.BuiltinEventEngine
	opts      *Options
	protoAddr string
	eng       gnet.Engine
}

func (m *baseListener) OnBoot(eng gnet.Engine) (action gnet.Action) {
	m.eng = eng
	return gnet.None
}
func (m *baseListener) Serve(eventHandler gnet.EventHandler) error {
	return gnet.Run(eventHandler, m.protoAddr, m.Options().opts...)
}

func (m *baseListener) Options() *Options {
	return m.opts
}

func (m *baseListener) Addr() string {
	return m.protoAddr
}

func (m *baseListener) onOpen(entity IEntity) (out []byte, action gnet.Action) {
	if err := m.opts.handler.OnOpen(entity); err != nil {
		return nil, gnet.Close
	}
	Add(entity)
	return nil, gnet.None
}

func (m *baseListener) onTraffic(entity IEntity) (action gnet.Action) {
	if err := m.opts.handler.OnTraffic(entity); err != nil {
		return gnet.Close
	}
	return gnet.None
}
func (m *baseListener) onClose(entity IEntity, err error) (action gnet.Action) {
	Remove(entity.ID())
	if wrong := m.opts.handler.OnClose(entity, err); wrong != nil {
		return gnet.Close
	}
	return gnet.None
}

func (m *baseListener) Close() error {
	return m.eng.Stop(context.Background())
}

// newTcpListener
//
//	@Description:
//	@param opts
//	@param protoAddr
//	@return *tcpListener
func newTcpListener(opts *Options, protoAddr string) *tcpListener {
	m := &tcpListener{
		baseListener: newBaseListener(opts, protoAddr),
	}
	return m
}

type tcpListener struct {
	*baseListener
}

func (m *tcpListener) Serve() error {
	return m.baseListener.Serve(m)
}

func (m *tcpListener) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	entity := newTcpEntity(m.opts, c)
	c.SetContext(entity)
	return m.baseListener.onOpen(entity)
}

func (m *tcpListener) OnTraffic(c gnet.Conn) (action gnet.Action) {
	entity := c.Context().(IEntity)
	if entity == nil {
		return gnet.Close
	}
	return m.baseListener.onTraffic(entity)
}

func (m *tcpListener) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	entity := c.Context().(IEntity)
	if entity == nil {
		return gnet.Close
	}
	return m.baseListener.onClose(entity, err)
}

// newUdpListener
//
//	@Description:
//	@param opts
//	@param protoAddr
//	@return *udpListener
func newUdpListener(opts *Options, protoAddr string) *udpListener {
	m := &udpListener{
		baseListener: newBaseListener(opts, protoAddr),
		dict:         make(map[string]IEntity),
	}
	return m
}

type udpListener struct {
	*baseListener
	dict map[string]IEntity
	sync.RWMutex
}

func (m *udpListener) Serve() error {
	return m.baseListener.Serve(m)
}
func (m *udpListener) OnTraffic(c gnet.Conn) (action gnet.Action) {
	remoteAddr := c.RemoteAddr().String()

	m.RLock()
	entity, ok := m.dict[remoteAddr]
	m.RUnlock()
	if ok {
		return m.baseListener.onTraffic(entity)
	}

	m.Lock()
	defer m.Unlock()
	entity, ok = m.dict[remoteAddr]
	if ok {
		return m.baseListener.onTraffic(entity)
	}

	entity = newUdpEntity(m.opts, c)
	m.baseListener.onOpen(entity)
	m.dict[remoteAddr] = entity
	return m.baseListener.onTraffic(entity)
}
