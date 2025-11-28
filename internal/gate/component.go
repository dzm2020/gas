package gate

import (
	"context"
	"gas/pkg/component"
	"gas/pkg/network"
)

// NewComponent 创建 Gate 组件
func NewComponent(address string, name string, factory Factory, opts ...network.Option) *Component {
	gate := New(address, factory, opts...)
	return &Component{
		gate: gate,
		name: name,
	}
}

// 确保 Component 实现了 Component 接口
var _ component.Component = (*Component)(nil)

// Component Gate 组件适配器
type Component struct {
	gate *Gate
	name string
}

// Name 返回组件名称
func (g *Component) Name() string {
	return g.name
}

// Start 启动 Gate 组件
func (g *Component) Start(ctx context.Context) error {
	return g.gate.Run()
}

// Stop 停止 Gate 组件
func (g *Component) Stop(ctx context.Context) error {
	return g.gate.Stop()
}
