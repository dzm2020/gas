package cluster

import (
	"context"
	"errors"
	"fmt"
	"time"

	dis "github.com/dzm2020/gas/pkg/discovery"
	discovery "github.com/dzm2020/gas/pkg/discovery/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib"
	"github.com/dzm2020/gas/pkg/lib/stopper"
	"github.com/dzm2020/gas/pkg/lib/xerror"
	mq "github.com/dzm2020/gas/pkg/messageQue"
	messageQue "github.com/dzm2020/gas/pkg/messageQue/iface"

	"go.uber.org/zap"
)

var (
	ErrNotFoundMember = errors.New("未找到成员节点")
)

type ICluster interface {
	Start(ctx context.Context) error

	Send(nodeId uint64, message interface{}) (err error)
	Call(nodeId uint64, message interface{}, timeout time.Duration) (data []byte, err error)
	Subscribe(nodeId uint64, subscriber messageQue.ISubscriber) (messageQue.ISubscription, error)

	Select(name string, strategy discovery.RouteStrategy) (uint64, error)
	Register(member *discovery.Member) error
	Update(member *discovery.Member) error
	Deregister(memberId uint64) error
	GetById(memberId uint64) *discovery.Member
	GetByKind(kind string) map[uint64]*discovery.Member
	GetByTag(tag string) []*discovery.Member
	GetAll() map[uint64]*discovery.Member
	Watch(kind string, handler discovery.ServiceChangeHandler)
	Unwatch(kind string, handler discovery.ServiceChangeHandler)

	Shutdown(ctx context.Context) error
}

var _ ICluster = (*Cluster)(nil)

func New(config *Config, serializer lib.ISerializer) (c *Cluster, err error) {
	conf := DefaultConfig()
	if config != nil {
		conf = config
	}
	c = &Cluster{
		serializer: serializer,
	}
	c.name = conf.Name
	// 创建服务发现实例
	c.IDiscovery, err = dis.NewFromConfig(*conf.Discovery)
	if err != nil {
		return
	}
	// 创建集群通信管理器
	c.mq, err = mq.NewFromConfig(*conf.MessageQueue)
	if err != nil {
		return
	}
	return
}

type Cluster struct {
	stopper.Stopper
	name       string
	serializer lib.ISerializer
	discovery.IDiscovery
	mq messageQue.IMessageQue
}

func (r *Cluster) Start(ctx context.Context) error {
	// 启动消息队列组件
	if err := r.mq.Run(ctx); err != nil {
		return err
	}
	// 启动服务发现组件
	if err := r.IDiscovery.Run(ctx); err != nil {
		return err
	}
	return nil
}

func (r *Cluster) Subscribe(nodeId uint64, subscriber messageQue.ISubscriber) (messageQue.ISubscription, error) {
	return r.mq.Subscribe(r.makeSubject(nodeId), subscriber)
}

func (r *Cluster) makeSubject(nodeId uint64) string {
	return fmt.Sprintf("%s.%d", r.name, nodeId)
}

// Send 发送消息到集群节点
func (r *Cluster) Send(nodeId uint64, message interface{}) (err error) {
	if m := r.IDiscovery.GetById(nodeId); m == nil {
		return xerror.Wrapf(ErrNotFoundMember, "nodeId=%d", nodeId)
	}
	bytes, mErr := r.serializer.Marshal(message)
	if mErr != nil {
		return mErr
	}

	subject := r.makeSubject(nodeId)

	return r.mq.Publish(subject, bytes)
}

func (r *Cluster) Call(nodeId uint64, message interface{}, timeout time.Duration) (bin []byte, err error) {
	if m := r.IDiscovery.GetById(nodeId); m == nil {
		err = xerror.Wrapf(ErrNotFoundMember, "nodeId=%d", nodeId)
		return
	}
	data, marshalErr := r.serializer.Marshal(message)
	if marshalErr != nil {
		return nil, marshalErr
	}
	subject := r.makeSubject(nodeId)
	return r.mq.Request(subject, data, timeout)
}

func (r *Cluster) Select(tag string, strategy discovery.RouteStrategy) (uint64, error) {
	if strategy == nil {
		strategy = discovery.RouteRandom
	}
	// 通过服务发现获取节点列表
	members := r.IDiscovery.GetByTag(tag)
	if len(members) == 0 {
		return 0, ErrNotFoundMember
	}

	// 使用路由策略选择节点
	selected := strategy(members)
	if selected == nil {
		return 0, ErrNotFoundMember
	}

	return selected.GetID(), nil
}

// Broadcast 向服务的所有节点广播消息
func (r *Cluster) Broadcast(tag string, message interface{}) {
	members := r.IDiscovery.GetByTag(tag)
	if len(members) == 0 {
		return
	}
	for _, member := range members {
		if err := r.Send(member.GetID(), message); err != nil {
			glog.Error("集群通信: 广播消息到节点失败",
				zap.Uint64("nodeId", member.GetID()),
				zap.String("tag", tag), zap.Error(err))
		}
	}
	return
}

// Shutdown 关闭所有订阅
func (r *Cluster) Shutdown(ctx context.Context) error {
	if !r.Stop() {
		return nil
	}
	if err := r.IDiscovery.Shutdown(ctx); err != nil {
		return err
	}
	if err := r.mq.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
