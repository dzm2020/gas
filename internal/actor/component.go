package actor

import (
	"context"
	"gas/internal/iface"
	"time"
)

// Component actor 系统组件适配器
type Component struct {
	system *System
	node   iface.INode
}

// NewComponent 创建 actor 组件
func NewComponent() *Component {
	return &Component{}
}

func (a *Component) Name() string {
	return "actorSystem"
}

func (a *Component) Start(ctx context.Context, node iface.INode) error {
	a.node = node
	a.system = NewSystem()
	a.system.SetSerializer(a.node.GetSerializer())
	a.system.SetNode(a.node)
	a.node.SetActorSystem(a.system)
	return nil
}

func (a *Component) Stop(ctx context.Context) error {
	if a.system == nil {
		return nil
	}
	a.node.SetActorSystem(nil)
	return a.system.Shutdown(10 * time.Second)
}
