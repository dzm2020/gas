package component

import (
	"context"

	"github.com/dzm2020/gas/internal/gate"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/profile"
	"github.com/dzm2020/gas/pkg/lib/component"
)

const (
	Gate = "gate"
)

type GateComponent struct {
	component.BaseComponent[iface.INode]
	*gate.Gate
}

// NewGateComponent 创建新的网关组件（使用默认配置）
func NewGateComponent() *GateComponent {
	c := &GateComponent{
		Gate: &gate.Gate{},
	}
	return c
}

func (r *GateComponent) Name() string {
	return Gate
}

func (r *GateComponent) Start(ctx context.Context, node iface.INode) error {
	conf := gate.DefaultConfig()
	if err := profile.Get(r.Name(), conf); err != nil {
		return err
	}

	r.Gate.Options = gate.ToOptions(conf)
	r.Gate.Address = conf.Address
	r.Gate.MaxConn = int64(conf.MaxConn)
	return r.Gate.Start(ctx, node)
}

func (r *GateComponent) Stop(ctx context.Context) error {
	return r.Gate.Stop(ctx)
}
