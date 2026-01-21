package cluster

import (
	"context"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/profile"
	dis "github.com/dzm2020/gas/pkg/discovery"
	"github.com/dzm2020/gas/pkg/lib/component"
	mq "github.com/dzm2020/gas/pkg/messageQue"
)

const (
	ComponentName = "cluster"
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
	return ComponentName
}

func (r *Component) Start(ctx context.Context, node iface.INode) (err error) {
	r.node = node

	conf := defaultConfig()
	if err = profile.Get(r.Name(), conf); err != nil {
		return err
	}

	r.name = conf.Name
	// 创建服务发现实例
	r.dis, err = dis.NewFromConfig(*conf.Discovery)
	if err != nil {
		return
	}
	// 创建集群通信管理器
	r.mq, err = mq.NewFromConfig(*conf.MessageQueue)
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
