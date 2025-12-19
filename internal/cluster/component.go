package cluster

import (
	"context"
	"gas/internal/iface"
	discoveryFactory "gas/pkg/discovery"
	messageQueFactory "gas/pkg/messageQue"
)

type Component struct {
	*Cluster
}

func NewComponent() *Component {
	c := &Component{}
	return c
}

func (r *Component) Name() string {
	return "cluster"
}

func (r *Component) Start(ctx context.Context, node iface.INode) error {
	config := node.GetConfig()
	// 创建服务发现实例
	discoveryInstance, err := discoveryFactory.NewFromConfig(*config.Remote.Discovery)
	if err != nil {
		return err
	}

	// 创建集群通信管理器
	messageQueue, err := messageQueFactory.NewFromConfig(*config.Remote.MessageQueue)
	if err != nil {
		return err
	}

	r.Cluster = New(node, discoveryInstance, messageQueue, config.Remote.SubjectPrefix)

	//  建立引用
	node.SetCluster(r.Cluster)
	//  注册节点并订阅
	if err = r.Cluster.Start(ctx); err != nil {
		return err
	}
	return nil
}

func (r *Component) Stop(ctx context.Context) error {
	r.node.SetCluster(nil)
	return r.Cluster.Shutdown(ctx)
}
