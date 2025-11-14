package gate

import (
	"gas/pkg/actor"
)

type Agent struct {
	actor.Actor
}

func (agent *Agent) OnSessionOpen(ctx actor.IContext, session *Session) error {
	return nil
}

func (agent *Agent) OnSessionClose(ctx actor.IContext, session *Session) error {
	return nil
}
