package component

import (
	"context"

	"github.com/dzm2020/gas/internal/actor"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/profile"
	"github.com/dzm2020/gas/pkg/lib/component"
)

const (
	SystemName = "system"
)

// NewSystem 创建 actor 组件
func NewSystem() *System {
	return &System{}
}

type System struct {
	component.BaseComponent[iface.INode]
	iface.ISystem
}

func (c *System) Name() string {
	return SystemName
}

func (c *System) Start(ctx context.Context, node iface.INode) error {
	if profile.IsSingleNodeMode() {
		c.ISystem = actor.NewSystem(node.GetID(), node.Serializer())
	} else {
		c.ISystem = actor.NewClusterSystem(node.GetID(), node.Serializer(), node.Cluster())
	}
	return nil
}

func (c *System) Stop(ctx context.Context) error {
	return c.ISystem.Shutdown()
}
