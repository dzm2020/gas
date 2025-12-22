package cluster

import (
	"context"
	"gas/internal/iface"
	dis "gas/pkg/discovery"
	"gas/pkg/lib/component"
	mq "gas/pkg/messageQue"
)

type Config struct {
	Name         string      `json:"name" yaml:"name"`
	Discovery    *dis.Config `json:"discovery" yaml:"discovery"`
	MessageQueue *mq.Config  `json:"messageQueue" yaml:"messageQueue"`
}

func defaultConfig() *Config {
	return &Config{
		Name: "",
		Discovery: &dis.Config{
			Type:   "consul",
			Config: nil,
		},

		MessageQueue: &mq.Config{
			Type:   "nats",
			Config: nil,
		},
	}
}

type Component struct {
	component.BaseComponent[iface.INode]
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
	config := defaultConfig()

	r.name = config.Name
	r.node = node

	if err = node.GetConfig(r.Name(), config); err != nil {
		return err
	}
	// 创建服务发现实例
	r.dis, err = dis.NewFromConfig(*config.Discovery)
	if err != nil {
		return
	}
	// 创建集群通信管理器
	r.mq, err = mq.NewFromConfig(*config.MessageQueue)
	if err != nil {
		return
	}

	//  建立引用
	node.SetCluster(r.Cluster)
	return r.Cluster.Start(ctx)
}

func (r *Component) Stop(ctx context.Context) error {
	r.node.SetCluster(nil)
	return r.Cluster.Shutdown(ctx)
}
