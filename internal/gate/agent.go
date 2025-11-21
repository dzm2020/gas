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
	connection *Connection
	ctx        iface.IContext
}

func (agent *Agent) OnConnectionOpen(ctx iface.IContext, connection *Connection) error {
	agent.connection = connection
	return nil
}

func (agent *Agent) OnConnectionClose(ctx iface.IContext) error {
	agent.connection = nil
	return nil
}

func (agent *Agent) SendMessage(msg *protocol.Message) error {
	if agent.connection == nil {
		return fmt.Errorf("gate: connection is nil")
	}
	return agent.connection.Send(msg)
}

func (agent *Agent) GetConnection() *Connection {
	return agent.connection
}

func (agent *Agent) Close() {
	if agent.connection == nil {
		return
	}
	_ = agent.connection.Close()
}
