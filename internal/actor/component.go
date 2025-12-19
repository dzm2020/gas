package actor

import (
	"context"
	"gas/internal/iface"
	"time"
)

// NewComponent 创建 actor 组件
func NewComponent() *Component {
	return &Component{}
}

type Component struct {
	*System
}

func (c *Component) Name() string {
	return "actorSystem"
}

func (c *Component) Start(ctx context.Context, node iface.INode) error {
	c.System = NewSystem(node)
	node.SetSystem(c.System)
	return nil
}

func (c *Component) Stop(ctx context.Context) error {
	c.node.SetSystem(nil)
	return c.System.Shutdown(10 * time.Second)
}
