package component

import (
	"context"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/profile"
	cluster2 "github.com/dzm2020/gas/pkg/cluster"
	"github.com/dzm2020/gas/pkg/lib/component"
)

type Component struct {
	component.BaseComponent[iface.INode]
	*cluster2.Cluster
	node iface.INode
}

func NewComponent() *Component {
	c := &Component{
		Cluster: &cluster2.Cluster{},
	}
	return c
}

func (r *Component) Name() string {
	return "cluster"
}

func (r *Component) Start(ctx context.Context, node iface.INode) (err error) {
	r.node = node
	conf := cluster2.DefaultConfig()
	if err = profile.Get(r.Name(), conf); err != nil {
		return err
	}
	r.Cluster, err = cluster2.New(conf, node)
	if err != nil {
		return err
	}
	//  建立引用
	node.SetCluster(r.Cluster)
	return r.Cluster.Start(ctx)
}

func (r *Component) Stop(ctx context.Context) error {
	r.node.SetCluster(nil)
	return r.Cluster.Shutdown(ctx)
}

//func (r *Cluster) OnMessage(data []byte, response func(data []byte) error) {
//	message := &iface.Message{}
//	var err error
//	defer func() {
//		if err != nil {
//			glog.Error("集群：处理消息失败", zap.Error(err), zap.Any("message", message))
//		}
//	}()
//	if err = r.serializer.Unmarshal(data, message); err != nil {
//		return
//	}
//	msg := &iface.ActorMessage{Message: message}
//
//	glog.Debug("集群：处理消息", zap.Any("message", message))
//
//	system := r.node.System()
//	if msg.GetAsync() {
//		err = system.Send(msg)
//	} else {
//		//  调用本地actor
//		responseData, responseErr := system.Call(msg)
//		//  打包结果
//		responseMessage := iface.NewResponse(responseData, responseErr)
//		responseData, err = r.node.Marshal(responseMessage)
//		if err != nil {
//			return
//		}
//		//  写入到消息队列
//		err = response(responseData)
//	}
//}
