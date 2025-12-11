package gate

import (
	"gas/internal/gate/protocol"
	"gas/internal/iface"
	"gas/pkg/lib/glog"
	"gas/pkg/network"
)

func handlerPush(ctx iface.IContext, session iface.ISession, data []byte) {
	agent := ctx.Actor().(IAgent)
	agent.Push(session, data)
}

func handlerClose(ctx iface.IContext, session iface.ISession, data []byte) {
	agent := ctx.Actor().(*Agent)
	if agent.IConnection == nil {
		//  todo 日志
		return
	}
	_ = agent.IConnection.Close(nil)
}

type Factory func() iface.IActor

type IAgent interface {
	iface.IActor
	OnConnect(ctx iface.IContext, connection network.IConnection) error
	OnClose(ctx iface.IContext) error
	Push(session iface.ISession, data []byte)
}

type Agent struct {
	iface.Actor
	network.IConnection
	ctx iface.IContext
}

func (agent *Agent) OnConnect(ctx iface.IContext, connection network.IConnection) error {

	agent.IConnection = connection

	router := ctx.GetRouter()
	if router == nil {
		return nil
	}
	router.Register(iface.MsgIdPushMessageToClient, handlerPush)
	router.Register(iface.MsgIdCloseClientConnection, handlerClose)
	return nil
}

func (agent *Agent) OnClose(ctx iface.IContext) error {
	agent.IConnection = nil
	_ = ctx.Process().Shutdown()
	return nil
}

func (agent *Agent) OnStop(ctx iface.IContext) error {
	if agent.IConnection != nil {
		_ = agent.IConnection.Close(nil)
	}
	return nil
}

func (agent *Agent) Push(session iface.ISession, data []byte) {
	cmd, act := protocol.ParseId(uint16(session.GetMid()))
	response := protocol.New(cmd, act, data)
	response.Index = session.GetIndex() // 可以根据需要设置 Index
	response.Error = uint16(session.GetCode())
	if err := agent.IConnection.Send(response); err != nil {
		glog.Errorf("send response error:%v", err)
	}
}
