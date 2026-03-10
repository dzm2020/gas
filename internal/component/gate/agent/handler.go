package agent

import (
	"github.com/dzm2020/gas/internal/component/gate/gateiface"
)

// Factory 创建每个连接对应的 IHandler，由 Gate 在 OnConnect 时调用。
type Factory func() IHandler

// IHandler 业务侧实现的接口：初始化、按消息路由、停止时清理。
type IHandler interface {
	OnInit(agent gateiface.IAgent) error
	OnRoute(agent gateiface.IAgent, data []byte) error
	OnStop(agent gateiface.IAgent) error
}

// Handler 空实现，便于业务只重写需要的方法。
type Handler struct{}

// OnInit
//
//	@Description: 空实现，可重写。
//	@receiver a
//	@param agent
//	@return error
func (a *Handler) OnInit(agent gateiface.IAgent) error {
	return nil
}

// OnRoute
//
//	@Description: 空实现，可重写。
//	@receiver a
//	@param agent
//	@param data
//	@return error
func (a *Handler) OnRoute(agent gateiface.IAgent, data []byte) error {
	return nil
}

// OnStop
//
//	@Description: 空实现，可重写。
//	@receiver a
//	@param agent
//	@return error
func (a *Handler) OnStop(agent gateiface.IAgent) error {
	return nil
}
