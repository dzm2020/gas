package iface

import (
	"context"
	"gas/internal/config"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/lib"

	"go.uber.org/zap/zapcore"
)

var currentNode INode

func SetNode(node INode) {
	currentNode = node
}

func GetNode() INode {
	return currentNode
}

type IComponentManager interface {
	ComponentCount() int
	GetComponent(name string) IComponent
	GetComponentNames() []string
	Register(component IComponent) error
}

type INode interface {
	IComponentManager
	discovery.IMember
	Info() *discovery.Member
	SetSerializer(ser lib.ISerializer)
	System() ISystem
	SetSystem(system ISystem)
	Remote() IRemote
	SetRemote(IRemote)
	GetConfig() *config.Config
	Startup(comps ...IComponent) error
	SetPanicHook(panicHook func(entry zapcore.Entry))
	CallPanicHook(entry zapcore.Entry)
	Marshal(request interface{}) []byte
	Unmarshal(data []byte, reply interface{})
}

// IComponent 组件接口
type IComponent interface {
	// Start 启动组件
	Start(ctx context.Context) error
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
