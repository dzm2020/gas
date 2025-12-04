package gate

import (
	"fmt"
	"gas/internal/gate/protocol"
	"gas/internal/iface"
	"gas/pkg/network"
)

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

func (agent *Agent) Send(msg *iface.Message) error {
	if agent.IConnection == nil {
		return fmt.Errorf("connection is nil")
	}
	// 创建响应消息（使用相同的 cmd 和 act）
	cmd, act := protocol.ParseId(uint16(msg.GetId()))
	responseMsg := protocol.New(cmd, act, msg.GetData())
	responseMsg.Index = 0 // 可以根据需要设置 Index
	return agent.IConnection.Send(responseMsg)
}
