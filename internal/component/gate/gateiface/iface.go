// Package gateiface 定义 gate 组件内 Agent 与 Middleware 的接口，供 middleware 与 agent 包共同依赖，避免循环引用。
package gateiface

import (
	"context"

	"github.com/dzm2020/gas/internal/component/gate/protocol"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/network"
)

type IGate interface {
	AppendOptions(options ...network.Option)
	SetMaximumOfConn(n int64)
	GetConnectionCount() int64
	SetSystem(system iface.ISystem)
	SetAddress(address string)
	Start(ctx context.Context) (err error)
	Stop(ctx context.Context) error
}

// IAgent 对外暴露的 Agent 能力：获取连接、Session、以及中间件链的配置（便于集群下通过 Actor 消息扩展）。
// 由 agent.Agent 实现；middleware 包依赖本接口以便 IMiddleware 方法接收 agent。
type IAgent interface {
	Context() iface.IContext
	GetEntity() network.IConnection
	GetSession() iface.ISession
	SetMiddleware(chain []IMiddleware)
	AppendMiddleware(middlewares ...IMiddleware)
	GetMiddleware() []IMiddleware
	Push(msg *protocol.Message) (err error)
	SetValues(values map[string]string) error
	Shutdown() error
}

// IMiddleware 在 codec.Decode 之后、以及 codec.Encode 之前对消息进行处理。
// 必须依赖 IAgent：AfterDecode/BeforeEncode 接收 IAgent，由调用方传入具体 Agent。
type IMiddleware interface {
	// AfterDecode Decode 之后调用，可修改或替换 msg，返回 error 会终止后续处理
	AfterDecode(agent IAgent, msg *protocol.Message) (*protocol.Message, error)
	// BeforeEncode Encode 之前调用，可修改或替换 msg，返回 error 会终止发送
	BeforeEncode(agent IAgent, msg *protocol.Message) (*protocol.Message, error)
}

// IBusinessHandler 业务侧实现的接口：初始化、按消息路由、停止时清理。
type IBusinessHandler interface {
	OnInit(agent IAgent) error
	OnRoute(agent IAgent, session iface.ISession, data []byte) error
	OnStop(agent IAgent) error
}

// AgentHandler 空实现，便于业务只重写需要的方法。
type AgentHandler struct{}

func (a *AgentHandler) OnInit(agent IAgent) error {
	return nil
}
func (a *AgentHandler) OnRoute(agent IAgent, session iface.ISession, data []byte) error {
	return nil
}
func (a *AgentHandler) OnStop(agent IAgent) error {
	return nil
}
