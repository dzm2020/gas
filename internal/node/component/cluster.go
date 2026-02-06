package component

import (
	"context"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/profile"
	"github.com/dzm2020/gas/pkg/cluster"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/component"
	"go.uber.org/zap"
)

const Cluster = "cluster"

type ClusterComponent struct {
	component.BaseComponent[iface.INode]
	cluster.ICluster
	node iface.INode
}

func NewClusterComponent() *ClusterComponent {
	c := &ClusterComponent{
		ICluster: &cluster.Cluster{},
	}
	return c
}

func (r *ClusterComponent) Name() string {
	return Cluster
}

func (r *ClusterComponent) Start(ctx context.Context, node iface.INode) (err error) {
	r.node = node
	//  创建cluster
	conf := cluster.DefaultConfig()
	if err = profile.Get(r.Name(), conf); err != nil {
		return err
	}
	r.ICluster, err = cluster.New(conf, node)

	if err != nil {
		return err
	}

	//  启动
	if err = r.ICluster.Run(ctx); err != nil {
		return err
	}
	//  注册
	if err = r.ICluster.Register(r.node.Info()); err != nil {
		return err
	}
	//  订阅
	if _, err = r.ICluster.Subscribe(r.node.GetID(), r); err != nil {
		return err
	}

	return
}

func (r *ClusterComponent) OnMessage(data []byte, response func(data []byte) error) {
	message := &iface.Message{}
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
		err = system.Send(msg)
	} else {
		//  调用本地actor
		responseData, responseErr := system.Call(msg)
		//  打包结果
		responseMessage := iface.NewResponse(responseData, responseErr)
		responseData, err = r.node.Marshal(responseMessage)
		if err != nil {
			return
		}
		//  写入到消息队列
		err = response(responseData)
	}
}

func (r *ClusterComponent) Stop(ctx context.Context) error {
	//  注销
	_ = r.Deregister(r.node.GetID())
	return r.ICluster.Shutdown(ctx)
}
