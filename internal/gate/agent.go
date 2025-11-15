package gate

import (
	"gas/pkg/actor"
)

type AgentProducer func() IAgent

type IAgent interface {
	actor.IActor
	OnSessionOpen(ctx actor.IContext, session *Session) error
	OnSessionClose(ctx actor.IContext, session *Session) error
}

type Agent struct {
	actor.Actor
}

func (agent *Agent) OnSessionOpen(ctx actor.IContext, session *Session) error {
	return nil
}

func (agent *Agent) OnSessionClose(ctx actor.IContext, session *Session) error {
	return nil
}
