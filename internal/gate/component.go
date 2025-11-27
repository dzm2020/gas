package gate

import (
	"context"
	"gas/pkg/component"
)

// NewComponent 创建 Gate 组件
func NewComponent(name string, factory Factory, opts ...Option) *Component {
	gate := New(factory, opts...)
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
	g.gate.Run(ctx)
	return nil
}

// Stop 停止 Gate 组件
func (g *Component) Stop(ctx context.Context) error {
	return g.gate.GracefulStop()
}
