package gate

import (
	"context"
	"gas/internal/iface"
	"gas/internal/profile"
	"gas/pkg/lib/component"
)

const (
	ComponentName = "gate"
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
	return ComponentName
}

func (r *Component) Start(ctx context.Context, node iface.INode) error {
	conf := defaultConfig()
	if err := profile.Get(r.Name(), conf); err != nil {
		return err
	}

	r.Gate.Options = ToOptions(conf)
	r.Gate.Address = conf.Address
	r.Gate.node = node
	r.Gate.maxConn = int64(conf.MaxConn)
	return r.Gate.Start(ctx)
}

func (r *Component) Stop(ctx context.Context) error {
	return r.Gate.Stop(ctx)
}
