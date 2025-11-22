package actor

import (
	"context"
	"gas/internal/iface"
	"time"
)

// Component actor 系统组件适配器
type Component struct {
	name   string
	system *System
	node   iface.INode
}

// NewComponent 创建 actor 组件
func NewComponent(name string, node iface.INode) *Component {
	return &Component{
		name: name,
		node: node,
	}
}

func (a *Component) Name() string {
	return a.name
}

func (a *Component) Start(ctx context.Context) error {
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
