package agent

import "github.com/dzm2020/gas/internal/iface"

type Factory func() IHandler

type IHandler interface {
	OnInit(ctx iface.IContext, agent IAgent) error
	OnRoute(ctx iface.IContext, agent IAgent, data []byte) error
	OnStop(ctx iface.IContext, agent IAgent) error
}
type Handler struct {
}

func (a *Handler) OnInit(ctx iface.IContext, agent IAgent) error {
	return nil
}

func (a *Handler) OnRoute(ctx iface.IContext, agent IAgent, data []byte) error {
	return nil
}

func (a *Handler) OnStop(ctx iface.IContext, agent IAgent) error {
	return nil
}
