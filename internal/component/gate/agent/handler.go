// Package agent 定义与 Gate 协作的 Agent：每个连接对应一个 Actor，实现 IHandler 处理业务路由，并响应 Push/SetValue/Shutdown 等系统调用。
package agent

import "github.com/dzm2020/gas/internal/iface"

// Factory 创建每个连接对应的 IHandler，由 Gate 在 OnConnect 时调用。
type Factory func() IHandler

// IHandler 业务侧实现的接口：初始化、按消息路由、停止时清理。
type IHandler interface {
	OnInit(ctx iface.IContext, agent IAgent) error
	OnRoute(ctx iface.IContext, agent IAgent, data []byte) error
	OnStop(ctx iface.IContext, agent IAgent) error
}

// Handler 空实现，便于业务只重写需要的方法。
type Handler struct{}

func (a *Handler) OnInit(ctx iface.IContext, agent IAgent) error {
	return nil
}

func (a *Handler) OnRoute(ctx iface.IContext, agent IAgent, data []byte) error {
	return nil
}

func (a *Handler) OnStop(ctx iface.IContext, agent IAgent) error {
	return nil
}
