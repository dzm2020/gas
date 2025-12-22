package cluster

import (
	"context"
	"gas/internal/iface"
	dis "gas/pkg/discovery"
	mq "gas/pkg/messageQue"
)

type Component struct {
	*Cluster
}

func NewComponent() *Component {
	c := &Component{
		Cluster: &Cluster{},
	}
	return c
}

func (r *Component) Name() string {
	return "cluster"
}

func (r *Component) Start(ctx context.Context, node iface.INode) (err error) {
	r.node = node
	config := node.GetConfig()
	clusterConfig := config.Cluster
	// 创建服务发现实例
	r.dis, err = dis.NewFromConfig(*clusterConfig.Discovery)
	if err != nil {
		return
	}
	// 创建集群通信管理器
	r.mq, err = mq.NewFromConfig(*clusterConfig.MessageQueue)
	if err != nil {
		return
	}
	r.name = config.Cluster.Name
	//  建立引用
	node.SetCluster(r.Cluster)
	return r.Cluster.Start(ctx)
}

func (r *Component) Stop(ctx context.Context) error {
	r.node.SetCluster(nil)
	return r.Cluster.Shutdown(ctx)
}
