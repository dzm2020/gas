package iface

import (
	"context"
	"gas/internal/config"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/lib"

	"go.uber.org/zap/zapcore"
)

type INode interface {
	GetID() uint64
	Info() *discovery.Member
	SetSerializer(ser lib.ISerializer)
	GetSerializer() lib.ISerializer
	GetActorSystem() ISystem
	SetActorSystem(system ISystem)
	GetRemote() IRemote
	SetRemote(IRemote)
	GetConfig() *config.Config
	StarUp(comps ...IComponent) error
	SetPanicHook(panicHook func(entry zapcore.Entry))
	GetTags() []string
	SetTags([]string)
}

// IComponent 组件接口
type IComponent interface {
	// Start 启动组件
	Start(ctx context.Context, node INode) error
	// Stop 停止组件，ctx 用于控制超时
	Stop(ctx context.Context) error
	// Name 返回组件名称，用于日志和错误报告
	Name() string
}

type BaseComponent struct {
}

func (*BaseComponent) Start(ctx context.Context) error {
	return nil
}
func (*BaseComponent) Stop(ctx context.Context) error {
	return nil
}
