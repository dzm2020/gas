package remote

import (
	"context"
	"fmt"
	"gas/internal/iface"
	discoveryFactory "gas/pkg/discovery"
	messageQueFactory "gas/pkg/messageQue"
)

// Component 远程通信组件
type Component struct {
	node   iface.INode
	name   string
	remote *Remote
}

// NewComponent 创建 remote 组件
func NewComponent(name string, node iface.INode) *Component {
	c := &Component{
		name: name,
		node: node,
	}

	return c
}

func (r *Component) Name() string {
	return r.name
}

func (r *Component) Start(ctx context.Context) error {
	if r.node == nil {
		return fmt.Errorf("game-node is nil")
	}

	config := r.node.GetConfig()
	// 创建服务发现实例
	discoveryInstance, err := discoveryFactory.NewFromConfig(config.Discovery)
	if err != nil {
		return fmt.Errorf("create discovery failed: %w", err)
	}

	// 创建远程通信管理器
	messageQueue, err := messageQueFactory.NewFromConfig(config.MessageQueue)
	if err != nil {
		return fmt.Errorf("create message queue failed: %w", err)
	}
	//  组合成remote
	nodeSubjectPrefix := config.Cluster.Name
	r.remote = New(messageQueue, discoveryInstance, nodeSubjectPrefix, r.node.GetSerializer())
	r.remote.SetSerializer(r.node.GetSerializer())
	r.remote.SetNode(r.node)
	//  注册节点并订阅
	if err = r.remote.registry(r.node.Self()); err != nil {
		return err
	}
	//  建立引用
	r.node.SetRemote(r.remote)
	return nil
}

func (r *Component) Stop(ctx context.Context) error {
	if r.node == nil {
		return nil
	}

	r.node.SetRemote(nil)

	return r.remote.Shutdown()
}
