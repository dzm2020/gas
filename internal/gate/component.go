package gate

import (
	"context"
	"fmt"
	"gas/internal/iface"
	"gas/pkg/component"
)

// NewComponent 创建 Gate 组件
func NewComponent(name string, node iface.INode, factory Factory, opts ...Option) *Component {
	gate := New(node, factory, opts...)
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
	if g.gate == nil {
		return fmt.Errorf("gate is nil")
	}
	g.gate.Run()
	return nil
}

// Stop 停止 Gate 组件
func (g *Component) Stop(ctx context.Context) error {
	if g.gate == nil {
		return nil
	}
	g.gate.GracefulStop()
	return nil
}
