package gate

import (
	"context"
	"gas/internal/iface"
)

type Component struct {
	*Gate
}

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
	r.Gate.node = node
	return r.Gate.Start(ctx)
}

func (r *Component) Stop(ctx context.Context) error {
	return r.Gate.Stop(ctx)
}
