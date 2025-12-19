package cluster

import (
	"context"
	"errors"
	"fmt"
	"gas/internal/iface"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/glog"
	"gas/pkg/lib"
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
		return fmt.Errorf("启动消息队列失败: %w", err)
	}
	// 启动服务发现组件
	if err := r.dis.Run(ctx); err != nil {
		return fmt.Errorf("启动服务发现失败: %w", err)
	}
	// 注册节点到服务发现
	if err := r.dis.Add(r.node.Info()); err != nil {
		return fmt.Errorf("注册节点失败: %w", err)
	}
	// 订阅消息队列
	if err := r.subscribe(); err != nil {
		return fmt.Errorf("订阅消息队列失败: %w", err)
	}
	return nil
}

func (r *Cluster) makeSubject(nodeId uint64) string {
	return fmt.Sprintf("%s%d", r.name, nodeId)
}

func (r *Cluster) subscribe() error {
	nodeId := r.node.GetID()
	subject := r.makeSubject(nodeId)
	_, err := r.mq.Subscribe(subject, r)
	if err != nil {
		return fmt.Errorf("订阅消息队列失败 (subject=%s, nodeId=%d): %w", subject, nodeId, err)
	}
	return nil
}

func (r *Cluster) HandlerAsyncMessage(data []byte) error {
	message := &iface.Message{}
	if err := r.node.Unmarshal(data, message); err != nil {
		return fmt.Errorf("反序列化异步消息失败: %w", err)
	}
	msg := &iface.ActorMessage{Message: message}
	system := r.node.System()
	if err := system.Send(msg); err != nil {
		return fmt.Errorf("发送异步消息失败: %w", err)
	}
	return nil
}

func (r *Cluster) HandlerSyncMessage(request []byte) (rspBytes []byte) {
	var bytes []byte
	var err error
	defer func() {
		response := iface.NewResponse(bytes, err)
		rspBytes, err = r.node.Marshal(response)
		if err != nil {
			glog.Error("序列化同步消息响应失败", zap.Error(err))
		}
	}()
	message := &iface.Message{}
	if err = r.node.Unmarshal(request, message); err != nil {
		err = fmt.Errorf("反序列化同步消息失败: %w", err)
		return
	}
	msg := &iface.ActorMessage{Message: message}
	system := r.node.System()
	bytes, err = system.Call(msg)
	if err != nil {
		err = fmt.Errorf("调用同步消息失败: %w", err)
	}
	return
}

// Send 发送消息到集群节点
func (r *Cluster) Send(msg *iface.ActorMessage) (err error) {
	if err = msg.Validate(); err != nil {
		return fmt.Errorf("消息验证失败: %w", err)
	}

	toNodeId := msg.To.GetNodeId()

	if m := r.dis.GetById(toNodeId); m == nil {
		return fmt.Errorf("%w: nodeId=%d", ErrNotFoundMember, toNodeId)
	}

	bytes, mErr := r.node.Marshal(msg)
	if mErr != nil {
		return fmt.Errorf("序列化消息失败: %w", mErr)
	}

	subject := r.makeSubject(toNodeId)
	if err = r.mq.Publish(subject, bytes); err != nil {
		return fmt.Errorf("发布消息到队列失败 (subject=%s): %w", subject, err)
	}
	return nil
}

func (r *Cluster) Call(msg *iface.ActorMessage) (bin []byte, err error) {
	if err = msg.Validate(); err != nil {
		return nil, fmt.Errorf("消息验证失败: %w", err)
	}

	toNodeId := msg.To.GetNodeId()

	if m := r.dis.GetById(toNodeId); m == nil {
		return nil, fmt.Errorf("%w: nodeId=%d", ErrNotFoundMember, toNodeId)
	}

	data, marshalErr := r.node.Marshal(msg.Message)
	if marshalErr != nil {
		return nil, fmt.Errorf("序列化消息失败: %w", marshalErr)
	}

	subject := r.makeSubject(toNodeId)
	timeout := lib.NowDelay(msg.GetDeadline(), 0)
	bytes, requestErr := r.mq.Request(subject, data, timeout)
	if requestErr != nil {
		return nil, fmt.Errorf("请求消息队列失败 (subject=%s, timeout=%v): %w", subject, timeout, requestErr)
	}

	response := &iface.Response{}
	if err = r.node.Unmarshal(bytes, response); err != nil {
		return nil, fmt.Errorf("反序列化响应失败: %w", err)
	}

	bin = response.GetData()
	err = response.GetError()
	if err != nil {
		err = fmt.Errorf("远程调用返回错误: %w", err)
	}
	return bin, err
}

func (r *Cluster) UpdateMember() error {
	if err := r.dis.Add(r.node.Info()); err != nil {
		return fmt.Errorf("更新成员信息失败: %w", err)
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
		return fmt.Errorf("关闭服务发现失败: %w", err)
	}
	if err := r.mq.Shutdown(ctx); err != nil {
		return fmt.Errorf("关闭消息队列失败: %w", err)
	}
	return nil
}
