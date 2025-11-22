package gate

import (
	"fmt"
	"gas/internal/gate/protocol"
	"gas/internal/iface"
)

type Factory func() IAgent

type IAgent interface {
	iface.IActor
	OnConnectionOpen(ctx iface.IContext, connection *Connection) error
	OnConnectionClose(ctx iface.IContext) error
}

type Agent struct {
	iface.Actor
	*Connection
	ctx iface.IContext
}

func (agent *Agent) OnConnectionOpen(ctx iface.IContext, connection *Connection) error {
	agent.Connection = connection
	return nil
}

func (agent *Agent) OnConnectionClose(ctx iface.IContext) error {
	agent.Connection = nil
	return nil
}

func (agent *Agent) GetConnection() *Connection {
	return agent.Connection
}

func (agent *Agent) Close() {
	if agent.Connection == nil {
		return
	}
	_ = agent.Connection.Close()
}

func (agent *Agent) Send(msg *iface.Message) error {
	if agent.Connection == nil {
		return fmt.Errorf("connection is nil")
	}
	// 创建响应消息（使用相同的 cmd 和 act）
	cmd, act := protocol.ParseId(uint16(msg.GetId()))
	responseMsg := protocol.New(cmd, act, msg.GetData())
	responseMsg.Index = 0 // 可以根据需要设置 Index
	return agent.Connection.Send(responseMsg)
}
