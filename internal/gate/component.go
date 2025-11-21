package gate

import (
	"context"
	"fmt"
	"gas/internal/iface"
	"gas/pkg/component"
)

// GateComponent Gate 组件适配器
type GateComponent struct {
	gate *Gate
	name string
}

// NewGateComponent 创建 Gate 组件
func NewGateComponent(name string, node iface.INode, factory Factory, opts ...Option) *GateComponent {
	gate := New(node, factory, opts...)
	return &GateComponent{
		gate: gate,
		name: name,
	}
}

// Name 返回组件名称
func (g *GateComponent) Name() string {
	return g.name
}

// Start 启动 Gate 组件
func (g *GateComponent) Start(ctx context.Context) error {
	if g.gate == nil {
		return fmt.Errorf("gate is nil")
	}
	g.gate.Run()
	return nil
}

// Stop 停止 Gate 组件
func (g *GateComponent) Stop(ctx context.Context) error {
	if g.gate == nil {
		return nil
	}

	// 使用 context 的超时，如果没有则使用默认超时
	stopCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		stopCtx, cancel = context.WithTimeout(ctx, g.gate.opts.GracePeriod)
		defer cancel()
	}

	// 在 goroutine 中执行优雅停止
	done := make(chan error, 1)
	go func() {
		g.gate.GracefulStop()
		done <- nil
	}()

	// 等待停止完成或超时
	select {
	case err := <-done:
		return err
	case <-stopCtx.Done():
		// 超时后强制关闭 listener
		if g.gate.listener != nil {
			_ = g.gate.listener.Close()
		}
		return stopCtx.Err()
	}
}

// GetGate 获取 Gate 实例
func (g *GateComponent) GetGate() *Gate {
	return g.gate
}

// 确保 GateComponent 实现了 Component 接口
var _ component.Component = (*GateComponent)(nil)
