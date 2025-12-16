package gate

import (
	"errors"
	"gas/internal/gate/protocol"
	"gas/internal/iface"
	"gas/pkg/network"
)

func (agent *Agent) PushMessageToClient(ctx iface.IContext, session iface.ISession, data []byte) error {
	entity := network.GetConnection(session.GetEntityId())
	if entity == nil {
		return errors.New("entity not found")
	}
	cmd, act := protocol.ParseId(uint16(session.GetMid()))
	response := protocol.New(cmd, act, data)
	response.Index = session.GetIndex()
	response.Error = uint16(session.GetCode())
	return entity.Send(response)

}

func (agent *Agent) CloseClientConnection(ctx iface.IContext, session iface.ISession, data []byte) error {
	entity := network.GetConnection(session.GetEntityId())
	if entity == nil {
		return errors.New("entity not found")
	}
	return entity.Close(nil)
}

type Factory func() iface.IActor

type IAgent interface {
	iface.IActor
	OnNetworkOpen(ctx iface.IContext, session iface.ISession) error
	OnNetworkMessage(ctx iface.IContext, session iface.ISession, data []byte) error
	OnNetworkClose(ctx iface.IContext, session iface.ISession) error
}

var _ IAgent = (*Agent)(nil)

type Agent struct {
	iface.Actor
}

func (agent *Agent) OnNetworkOpen(ctx iface.IContext, session iface.ISession) error {
	return nil
}
func (agent *Agent) OnNetworkMessage(ctx iface.IContext, session iface.ISession, data []byte) error {
	return nil
}
func (agent *Agent) OnNetworkClose(ctx iface.IContext, session iface.ISession) error {
	return nil
}
