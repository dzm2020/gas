package cluster

import (
	"context"

	"github.com/dzm2020/gas/internal/iface"
	pkgcluster "github.com/dzm2020/gas/pkg/cluster"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/component"
	"github.com/dzm2020/gas/pkg/lib/xerror"
	"go.uber.org/zap"
)

const Name = "cluster"

type Cluster struct {
	component.BaseComponent[iface.INode]
	pkgcluster.ICluster
	node iface.INode
}

func New() *Cluster {
	c := &Cluster{
		ICluster: &pkgcluster.Cluster{},
	}
	return c
}

func (r *Cluster) Name() string {
	return Name
}

func (r *Cluster) Start(ctx context.Context, node iface.INode) (err error) {
	r.node = node

	conf := node.Profile().GetCluster()

	r.ICluster, err = pkgcluster.New(conf, node.Serializer())

	if err != nil {
		return err
	}

	//  启动
	if err = r.ICluster.Run(ctx); err != nil {
		return xerror.Wrapf(err, "cluster run fail")
	}

	return
}

func (r *Cluster) Stop(ctx context.Context) error {
	if err := r.Deregister(r.node.GetID()); err != nil {
		glog.Debug("集群注销节点失败", zap.Uint64("nodeId", r.node.GetID()), zap.Error(err))
	}
	return r.ICluster.Shutdown(ctx)
}
