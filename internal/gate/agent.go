package gate

import (
	"errors"
	"gas/internal/gate/protocol"
	"gas/internal/iface"
	"gas/internal/session"
	"gas/pkg/network"
)

var (
	ErrNotFoundEntity = errors.New("entity not found")
)

type Factory func() iface.IActor

type IAgent interface {
	iface.IActor
	OnNetworkOpen(ctx iface.IContext, s *session.Session) error
	OnNetworkMessage(ctx iface.IContext, s *session.Session, data []byte) error
	OnNetworkClose(ctx iface.IContext, s *session.Session) error
}

var _ IAgent = (*Agent)(nil)

type Agent struct {
	iface.Actor
}

func (agent *Agent) OnNetworkOpen(ctx iface.IContext, s *session.Session) error {
	return nil
}
func (agent *Agent) OnNetworkMessage(ctx iface.IContext, s *session.Session, data []byte) error {
	return nil
}
func (agent *Agent) OnNetworkClose(ctx iface.IContext, s *session.Session) error {
	return nil
}
func (agent *Agent) PushMessageToClient(ctx iface.IContext, s *session.Session, data []byte) error {
	entity := network.GetConnection(s.GetEntityId())
	if entity == nil {
		return ErrNotFoundEntity
	}
	msg := protocol.New(uint8(s.Cmd), uint8(s.Act), data)
	msg.Index = s.GetIndex()
	msg.Error = uint16(s.GetCode())
	return entity.Send(msg)

}
func (agent *Agent) CloseClientConnection(ctx iface.IContext, s *session.Session, data []byte) error {
	entity := network.GetConnection(s.GetEntityId())
	if entity == nil {
		return ErrNotFoundEntity
	}
	return entity.Close(nil)
}
