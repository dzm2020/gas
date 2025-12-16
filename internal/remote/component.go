package remote

import (
	"context"
	"gas/internal/iface"
	discoveryFactory "gas/pkg/discovery"
	messageQueFactory "gas/pkg/messageQue"
)

// Component 远程通信组件
type Component struct {
	node   iface.INode
	remote *Remote
}

// NewComponent 创建 remote 组件
func NewComponent() *Component {
	c := &Component{}
	return c
}

func (r *Component) Name() string {
	return "remote"
}

func (r *Component) Start(ctx context.Context, node iface.INode) error {
	r.node = node

	config := r.node.GetConfig()
	// 创建服务发现实例
	discoveryInstance, err := discoveryFactory.NewFromConfig(config.Remote.Discovery)
	if err != nil {
		return err
	}

	// 创建远程通信管理器
	messageQueue, err := messageQueFactory.NewFromConfig(config.Remote.MessageQueue)
	if err != nil {
		return err
	}

	r.remote = &Remote{
		nodeSubjectPrefix: config.Remote.SubjectPrefix,
		discovery:         discoveryInstance,
		messageQue:        messageQueue,
		node:              r.node,
	}
	//  建立引用
	r.node.SetRemote(r.remote)
	//  注册节点并订阅
	if err = r.remote.Start(ctx); err != nil {
		return err
	}
	return nil
}

func (r *Component) Stop(ctx context.Context) error {
	if r.node == nil {
		return nil
	}

	r.node.SetRemote(nil)

	return r.remote.Shutdown(ctx)
}
