package network

import (
	"github.com/panjf2000/gnet/v2"
)

type IListener interface {
	Serve() error
}

type IEntityBehavior interface {
	OnOpen(entity IEntity) (out []byte, action gnet.Action)
	OnTraffic(entity IEntity) (action gnet.Action)
	OnClose(entity IEntity, _ error) (action gnet.Action)
}

type IEntity interface {
	IEntityBehavior
	ID() uint64
	Write(data []byte) error
	Network() string
	LocalAddr() string
	RemoteAddr() string
}

var _ IEntityBehavior = (*EntityBehavior)(nil)

type EntityBehavior struct {
}

func (e *EntityBehavior) OnOpen(entity IEntity) (out []byte, action gnet.Action) {
	return nil, gnet.None
}

func (e *EntityBehavior) OnTraffic(entity IEntity) (action gnet.Action) {
	return gnet.None
}

func (e *EntityBehavior) OnClose(entity IEntity, _ error) (action gnet.Action) {
	return gnet.None
}
