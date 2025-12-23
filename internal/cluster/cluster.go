package cluster

import (
	"context"
	"errors"
	"fmt"
	"gas/internal/iface"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/glog"
	"gas/pkg/lib"
	"gas/pkg/lib/xerror"
	messageQue "gas/pkg/messageQue/iface"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

var (
	ErrMethodNotImplemented = errors.New("集群推送任务尚未实现")
	ErrNotFoundMember       = errors.New("未找到成员节点")
)

var _ iface.ICluster = (*Cluster)(nil)

type Cluster struct {
	name string
	node iface.INode
	dis  discovery.IDiscovery
	mq   messageQue.IMessageQue
}

func (r *Cluster) PushTask(pid *iface.Pid, f iface.Task) error {
	return ErrMethodNotImplemented
}

func (r *Cluster) PushTaskAndWait(pid *iface.Pid, timeout time.Duration, task iface.Task) error {
	return ErrMethodNotImplemented
}

func (r *Cluster) Start(ctx context.Context) error {
	// 启动消息队列组件
	if err := r.mq.Run(ctx); err != nil {
		return err
	}
	// 启动服务发现组件
	if err := r.dis.Run(ctx); err != nil {
		return err
	}
	// 注册节点到服务发现
	if err := r.dis.Add(r.node.Info()); err != nil {
		return err
	}
	// 订阅消息队列
	if err := r.subscribe(); err != nil {
		return err
	}
	return nil
}

func (r *Cluster) makeSubject(nodeId uint64) string {
	return fmt.Sprintf("%s.%d", r.name, nodeId)
}

func (r *Cluster) subscribe() error {
	nodeId := r.node.GetID()
	subject := r.makeSubject(nodeId)
	_, err := r.mq.Subscribe(subject, r)
	if err != nil {
		return xerror.Wrapf(err, "订阅消息队列失败 (subject=%s, nodeId=%d)", subject, nodeId)
	}
	return nil
}

func (r *Cluster) OnMessage(data []byte) ([]byte, error) {
	message := &iface.Message{}
	if err := r.node.Unmarshal(data, message); err != nil {
		return nil, err
	}
	msg := &iface.ActorMessage{Message: message}
	if msg.GetAsync() {
		return nil, r.OnAsyncMessage(msg)
	} else {
		return r.OnSyncMessage(msg), nil
	}
}

func (r *Cluster) OnAsyncMessage(msg *iface.ActorMessage) error {
	system := r.node.System()
	if err := system.Send(msg); err != nil {
		return err
	}
	return nil
}

func (r *Cluster) OnSyncMessage(msg *iface.ActorMessage) (rspBytes []byte) {
	var bytes []byte
	var err error
	defer func() {
		response := iface.NewResponse(bytes, err)
		rspBytes, err = r.node.Marshal(response)
		if err != nil {
			glog.Error("序列化同步消息响应失败", zap.Error(err))
		}
	}()
	system := r.node.System()
	bytes, err = system.Call(msg)
	return
}

// Send 发送消息到集群节点
func (r *Cluster) Send(msg *iface.ActorMessage) (err error) {
	if err = msg.Validate(); err != nil {
		return err
	}

	toNodeId := msg.To.GetNodeId()

	if m := r.dis.GetById(toNodeId); m == nil {
		return xerror.Wrapf(ErrNotFoundMember, "nodeId=%d", toNodeId)
	}

	bytes, mErr := r.node.Marshal(msg)
	if mErr != nil {
		return mErr
	}

	subject := r.makeSubject(toNodeId)
	if err = r.mq.Publish(subject, bytes); err != nil {
		return xerror.Wrapf(err, "发布消息到队列失败 (subject=%s)", subject)
	}
	return nil
}

func (r *Cluster) Call(msg *iface.ActorMessage) (bin []byte, err error) {
	if err = msg.Validate(); err != nil {
		return
	}

	toNodeId := msg.To.GetNodeId()

	if m := r.dis.GetById(toNodeId); m == nil {
		err = xerror.Wrapf(ErrNotFoundMember, "nodeId=%d", toNodeId)
		return
	}

	data, marshalErr := r.node.Marshal(msg.Message)
	if marshalErr != nil {
		return nil, marshalErr
	}

	subject := r.makeSubject(toNodeId)
	timeout := lib.NowDelay(msg.GetDeadline(), 0)
	bytes, requestErr := r.mq.Request(subject, data, timeout)
	if requestErr != nil {
		err = xerror.Wrapf(requestErr, "请求消息队列失败 (subject=%s, timeout=%v)", subject, timeout)
		return
	}

	response := &iface.Response{}
	if err = r.node.Unmarshal(bytes, response); err != nil {
		return nil, err
	}

	bin = response.GetData()
	err = response.GetError()

	return bin, err
}

func (r *Cluster) UpdateMember() error {
	if err := r.dis.Add(r.node.Info()); err != nil {
		return err
	}
	return nil
}

func (r *Cluster) Select(tag string, strategy discovery.RouteStrategy) uint64 {
	if strategy == nil {
		strategy = discovery.RouteRandom
	}
	// 通过服务发现获取节点列表
	members := r.dis.GetAll()
	if len(members) == 0 {
		return 0
	}

	var selected []*discovery.Member
	for _, node := range members {
		if !slices.Contains(node.Tags, tag) {
			continue
		}
		selected = append(selected, node)
	}

	// 使用路由策略选择节点
	selectedNode := strategy(selected)
	if selectedNode == nil {
		return 0
	}

	return selectedNode.GetID()
}

// Broadcast 向服务的所有节点广播消息
func (r *Cluster) Broadcast(tag string, message *iface.ActorMessage) {
	members := r.dis.GetAll()
	if len(members) == 0 {
		return
	}
	for _, member := range members {
		if !slices.Contains(member.Tags, tag) {
			continue
		}
		msg := convertor.DeepClone(message)
		msg.To = &iface.Pid{
			NodeId: member.GetID(),
			Name:   tag,
		}
		// 发送消息
		if err := r.Send(msg); err != nil {
			glog.Error("集群通信: 广播消息到节点失败",
				zap.Uint64("nodeId", member.GetID()),
				zap.String("tag", tag), zap.Error(err))
		}
	}
	return
}

// Shutdown 关闭所有订阅
func (r *Cluster) Shutdown(ctx context.Context) error {
	if err := r.dis.Shutdown(ctx); err != nil {
		return xerror.Wrap(err, "关闭服务发现失败")
	}
	if err := r.mq.Shutdown(ctx); err != nil {
		return xerror.Wrap(err, "关闭消息队列失败")
	}
	return nil
}
