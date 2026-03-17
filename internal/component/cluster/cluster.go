package cluster

import (
	"context"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/pb"
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
	if node.Profile().IsSingleNodeMode() {
		return
	}
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

	//  订阅
	if _, err = r.ICluster.Subscribe(r.node.GetID(), r); err != nil {
		return xerror.Wrapf(err, "message queue subscribe fail")
	}

	return
}

func (r *Cluster) OnMessage(data []byte, response func(data []byte) error) {
	message := &pb.Message{}
	var err error
	defer func() {
		if err != nil {
			glog.Error("集群：处理消息失败", zap.Error(err), zap.Any("message", message))
		}
	}()

	serializer := r.node.Serializer()
	if err = serializer.Unmarshal(data, message); err != nil {
		return
	}
	msg := &iface.ActorMessage{Message: message}

	glog.Debug("集群：处理消息", zap.Any("message", message))

	system := r.node.System()
	if msg.GetAsync() {
		err = system.SendMessage(msg)
	} else {
		//  调用本地 actor（传输层已有完整 Message，使用 CallMessage）
		responseData, responseErr := system.CallMessage(msg)
		//  打包结果
		responseMessage := iface.NewResponse(responseData, responseErr)
		responseData, err = r.node.Serializer().Marshal(responseMessage)
		if err != nil {
			return
		}
		//  写入到消息队列
		err = response(responseData)
	}
}

func (r *Cluster) Stop(ctx context.Context) error {
	//  注销
	_ = r.Deregister(r.node.GetID())
	return r.ICluster.Shutdown(ctx)
}
