package service

import "gas/pkg/actor"

type Service struct {
	actor.Actor
}

func (a *Service) OnMessage(ctx actor.IContext, msg interface{}) error {
	return nil
}
