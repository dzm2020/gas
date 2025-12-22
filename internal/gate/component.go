package gate

import (
	"context"
	"gas/internal/iface"
	"gas/pkg/lib/component"
)

type Component struct {
	component.BaseComponent[iface.INode]
	*Gate
}

// NewComponent 创建新的网关组件（使用默认配置）
func NewComponent() *Component {
	c := &Component{
		Gate: &Gate{},
	}
	return c
}

func (r *Component) Name() string {
	return "gate"
}

func (r *Component) Start(ctx context.Context, node iface.INode) error {
	config := defaultConfig()
	if err := node.GetConfig(r.Name(), config); err != nil {
		return err
	}

	r.Gate.Options = ToOptions(config)
	r.Gate.Address = config.Address
	r.Gate.node = node
	return r.Gate.Start(ctx)
}

func (r *Component) Stop(ctx context.Context) error {
	return r.Gate.Stop(ctx)
}
