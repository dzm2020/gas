package component

import (
	"context"
	"fmt"

	"github.com/dzm2020/gas/internal/component/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/pb"
	"github.com/dzm2020/gas/internal/profile"
	"github.com/dzm2020/gas/pkg/cluster"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/component"
	"github.com/dzm2020/gas/pkg/lib/xerror"
	"go.uber.org/zap"
)

const ClusterName = "cluster"

type Cluster struct {
	component.BaseComponent[iface.INode]
	cluster.ICluster
	node iface.INode
}

func NewCluster() *Cluster {
	c := &Cluster{
		ICluster: &cluster.Cluster{},
	}
	return c
}

func (r *Cluster) Name() string {
	return ClusterName
}

func (r *Cluster) Start(ctx context.Context, node iface.INode) (err error) {
	if profile.IsSingleNodeMode() {
		return
	}
	r.node = node

	conf := profile.GetCluster()

	r.ICluster, err = cluster.New(conf, node.Serializer())

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

	fmt.Printf("Cluster Send OnMessage:  data%v \n", []byte(data))

	serializer := r.node.Serializer()
	if err = serializer.Unmarshal(data, message); err != nil {
		return
	}
	msg := &iface.ActorMessage{Message: message}

	sdata := msg.Session.Values[session.KeyMessage]
	fmt.Printf("Cluster Send OnMessage:%v \n", []byte(sdata))
	glog.Info("集群：处理消息", zap.Any("message", message))

	system := r.node.System()
	if msg.GetAsync() {
		err = system.Send(msg)
	} else {
		//  调用本地actor
		responseData, responseErr := system.Call(msg)
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
