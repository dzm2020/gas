package actor

import (
	"context"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/lib/component"
)

const (
	ComponentName = "system"
)

// NewComponent 创建 actor 组件
func NewComponent() *Component {
	return &Component{}
}

type Component struct {
	component.BaseComponent[iface.INode]
	*System
}

func (c *Component) Name() string {
	return ComponentName
}

func (c *Component) Start(ctx context.Context, node iface.INode) error {
	c.System = NewSystem(node)
	node.SetSystem(c.System)
	return nil
}

func (c *Component) Stop(ctx context.Context) error {
	c.node.SetSystem(nil)
	return c.System.Shutdown()
}
