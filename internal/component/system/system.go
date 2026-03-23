package system

import (
	"context"

	"github.com/dzm2020/gas/internal/actor"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/lib/component"
)

const Name = "system"

// New 创建 actor 组件
func New() *System {
	return &System{}
}

type System struct {
	component.BaseComponent[iface.INode]
	iface.ISystem
}

func (c *System) Name() string {
	return Name
}

func (c *System) Start(ctx context.Context, node iface.INode) error {
	if node.Profile().IsSingleNodeMode() {
		c.ISystem = actor.NewSystem(node.GetID(), node.Serializer())
		return nil
	}
	cs, err := actor.NewClusterSystem(node.GetID(), node.Serializer(), node.Cluster())
	if err != nil {
		return err
	}
	c.ISystem = cs
	return nil
}

func (c *System) Stop(ctx context.Context) error {
	return c.ISystem.Shutdown()
}
