package gate

import (
	"gas/internal/gate/protocol"
	"gas/internal/iface"
	"gas/pkg/network"
)

func handlerPush(ctx iface.IContext, session *iface.Session, data []byte) {
	agent := ctx.Actor().(*Agent)

	if agent.IConnection == nil {
		//  todo 日志
		return
	}

	agent.Push(session, data)
}

func handlerClose(ctx iface.IContext, session *iface.Session, data []byte) {
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
	_ = router.Register(-1, handlerPush)
	_ = router.Register(-2, handlerClose)
	return nil
}

func (agent *Agent) OnClose(ctx iface.IContext) error {
	agent.IConnection = nil
	ctx.Exit()
	return nil
}

func (agent *Agent) OnStop(ctx iface.IContext) error {
	if agent.IConnection != nil {
		_ = agent.IConnection.Close(nil)
	}
	return nil
}

func (agent *Agent) Push(session *iface.Session, data []byte) {
	// 创建响应消息（使用相同的 cmd 和 act）
	cmd, act := protocol.ParseId(uint16(session.GetMid()))
	response := protocol.New(cmd, act, data)
	response.Index = session.GetIndex() // 可以根据需要设置 Index
	if err := agent.IConnection.Send(response); err != nil {
		// todo 日志
	}
}
