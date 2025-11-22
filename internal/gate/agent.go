package gate

import (
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
