package gate

import (
	"context"
	"gas/internal/iface"
)

type Component struct {
	*Gate
}

// NewComponent 创建新的网关组件（使用默认配置）
func NewComponent() *Component {
	c := &Component{
		Gate: &Gate{},
	}
	return c
}

// NewComponentFromConfig 从配置创建网关组件
func NewComponentFromConfig() *Component {
	c := &Component{
		Gate: &Gate{},
	}
	return c
}

func (r *Component) Name() string {
	return "gate"
}

func (r *Component) Start(ctx context.Context, node iface.INode) error {
	c := node.GetConfig().Gate
	r.Gate.Options = ToOptions(c)
	r.Gate.Address = c.Address
	r.Gate.node = node
	return r.Gate.Start(ctx)
}

func (r *Component) Stop(ctx context.Context) error {
	return r.Gate.Stop(ctx)
}
