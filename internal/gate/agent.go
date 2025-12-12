package gate

import (
	"errors"
	"gas/internal/gate/protocol"
	"gas/internal/iface"
	"gas/pkg/network"
)

func (agent *Agent) handlerPush(ctx iface.IContext, session iface.ISession, data []byte) error {
	entity := network.GetConnection(session.GetEntityId())
	if entity == nil {
		return errors.New("entity not found")
	}
	cmd, act := protocol.ParseId(uint16(session.GetMid()))
	response := protocol.New(cmd, act, data)
	response.Index = session.GetIndex() // 可以根据需要设置 Index
	response.Error = uint16(session.GetCode())
	return entity.Send(response)

}

func handlerClose(ctx iface.IContext, session iface.ISession, data []byte) error {
	//agent := ctx.Actor().(*Agent)
	//if agent.IConnection == nil {
	//	return nil
	//}
	//return agent.IConnection.Close(nil)
}

type Factory func() iface.IActor

type IAgent interface {
	iface.IActor
	OnConnectionOpen(ctx iface.IContext, connection network.IConnection) error
	OnConnectionClose(ctx iface.IContext) error
}

type Agent struct {
	iface.Actor
	ctx iface.IContext
}

func (agent *Agent) OnConnect(ctx iface.IContext, connection network.IConnection) error {
	router := ctx.GetRouter()
	if router == nil {
		return nil
	}
	router.Register(iface.MsgIdPushMessageToClient, handlerPush)
	router.Register(iface.MsgIdCloseClientConnection, handlerClose)

	ctx.System().PushTask(pid, handlerPush, arg1, arg2)

	return nil
}
