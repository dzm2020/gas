package node

import (
	"context"
	"fmt"
	"time"

	"gas/internal/actor"
	discovery "gas/pkg/discovery/iface"
	messageQue "gas/pkg/messageQue/iface"
	"gas/pkg/utils/glog"
)

// DiscoveryComponent discovery 组件适配器
type DiscoveryComponent struct {
	discovery discovery.IDiscovery
	name      string
}

func NewDiscoveryComponent(name string, discovery discovery.IDiscovery) *DiscoveryComponent {
	return &DiscoveryComponent{
		name:      name,
		discovery: discovery,
	}
}

func (d *DiscoveryComponent) Name() string {
	return d.name
}

func (d *DiscoveryComponent) Start(ctx context.Context) error {
	if d.discovery == nil {
		return fmt.Errorf("discovery is nil")
	}
	return nil
}

func (d *DiscoveryComponent) Stop(ctx context.Context) error {
	if d.discovery == nil {
		return nil
	}
	return d.discovery.Close()
}

// MessageQueComponent messageQue 组件适配器
type MessageQueComponent struct {
	messageQue messageQue.IMessageQue
	name       string
}

func NewMessageQueComponent(name string, messageQue messageQue.IMessageQue) *MessageQueComponent {
	return &MessageQueComponent{
		name:       name,
		messageQue: messageQue,
	}
}

func (m *MessageQueComponent) Name() string {
	return m.name
}

func (m *MessageQueComponent) Start(ctx context.Context) error {
	if m.messageQue == nil {
		return fmt.Errorf("messageQue is nil")
	}
	return nil
}

func (m *MessageQueComponent) Stop(ctx context.Context) error {
	if m.messageQue == nil {
		return nil
	}
	return m.messageQue.Close()
}

// RemoteComponent 远程通信组件
type RemoteComponent struct {
	node *Node
	name string
}

func NewRemoteComponent(name string, node *Node) *RemoteComponent {
	return &RemoteComponent{
		name: name,
		node: node,
	}
}

func (r *RemoteComponent) Name() string {
	return r.name
}

func (r *RemoteComponent) Start(ctx context.Context) error {
	if r.node == nil {
		return fmt.Errorf("node is nil")
	}
	// 设置 Actor 系统到 Remote
	if r.node.remote != nil {
		r.node.remote.SetNode(r.node)
	}
	return nil
}

func (r *RemoteComponent) Stop(ctx context.Context) error {
	if r.node == nil {
		return nil
	}

	// 注销节点
	if r.node.node != nil {
		_ = r.node.unregisterNode(r.node.node.GetID())
	}

	return nil
}

// ActorComponent actor 系统组件适配器
type ActorComponent struct {
	nodeId uint64
	node   *Node
	name   string
}

func NewActorComponent(name string, nodeId uint64, node *Node) *ActorComponent {
	return &ActorComponent{
		name:   name,
		nodeId: nodeId,
		node:   node,
	}
}

func (a *ActorComponent) Name() string {
	return a.name
}

func (a *ActorComponent) Start(ctx context.Context) error {
	// 创建 Actor 系统
	system := actor.NewSystem()
	system.SetNode(a.node)

	// 保存到 Node 中
	a.node.actorSystem = system
	return nil
}

func (a *ActorComponent) Stop(ctx context.Context) error {
	if a.node.actorSystem == nil {
		return nil
	}

	// 优雅关闭所有 actor 进程
	for _, process := range a.node.actorSystem.GetAllProcesses() {
		if err := process.Exit(); err != nil {
			glog.Errorf("actor: exit process failed: %v", err)
		}
	}

	// 等待进程完成清理
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}

	// 清理引用
	a.node.actorSystem = nil
	return nil
}
