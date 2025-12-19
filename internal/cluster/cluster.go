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
	MethodNotImplemented = errors.New("cluster push task not yet implemented")
	NotFoundMember       = errors.New("not found member")
)

var _ iface.ICluster = (*Cluster)(nil)

// New 创建集群通信管理器
func New(node iface.INode, discovery discovery.IDiscovery, messageQue messageQue.IMessageQue, subjectPrefix string) *Cluster {
	return &Cluster{
		node:       node,
		discovery:  discovery,
		messageQue: messageQue,
		prefix:     subjectPrefix,
	}
}

// Cluster 集群通信管理器，负责处理跨节点的消息传递
type Cluster struct {
	discovery  discovery.IDiscovery
	messageQue messageQue.IMessageQue
	prefix     string
	node       iface.INode // 保存 node 引用，避免全局状态
}

func (r *Cluster) PushTask(pid *iface.Pid, f iface.Task) error {
	return MethodNotImplemented
}

func (r *Cluster) PushTaskAndWait(pid *iface.Pid, timeout time.Duration, task iface.Task) error {
	return MethodNotImplemented
}

func (r *Cluster) Start(ctx context.Context) error {
	if err := r.messageQue.Run(ctx); err != nil {
		return err
	}
	if err := r.discovery.Run(ctx); err != nil {
		return err
	}
	if err := r.discovery.Add(r.node.Info()); err != nil {
		return err
	}
	if err := r.init(); err != nil {
		return err
	}
	return nil
}

func (r *Cluster) makeTopic(nodeId uint64) string {
	return fmt.Sprintf("%s%d", r.prefix, nodeId)
}

func (r *Cluster) init() error {
	nodeId := r.node.GetID()
	handler := func(data []byte, reply func([]byte) error) {
		if err := r.onRemoteProcess(data, reply); err != nil {
			glog.Error("集群通信: 处理集群消息失败", zap.Error(err))
		}
	}
	subject := r.makeTopic(nodeId)
	_, err := r.messageQue.Subscribe(subject, handler)
	return err
}

// onRemoteProcess 处理集群消息
func (r *Cluster) onRemoteProcess(data []byte, reply func([]byte) error) error {
	system := r.node.System()

	message := &iface.Message{}
	if err := r.node.Unmarshal(data, message); err != nil {
		_ = r.response(reply, err, nil)
		return err
	}

	glog.Debug("集群通信: 处理集群消息", zap.Any("message", message))

	msg := &iface.ActorMessage{Message: message}

	if msg.GetAsync() {
		return system.Send(msg)
	}

	bin, err := system.Call(msg)
	if err != nil {
		_ = r.response(reply, err, nil)
		return err
	}
	return r.response(reply, err, bin)
}

func (r *Cluster) response(reply func([]byte) error, err error, data []byte) error {
	if reply == nil {
		return nil
	}

	response := &iface.Response{
		Data: data,
	}
	if err != nil {
		response.ErrMsg = err.Error()
	}

	bytes, marshalErr := r.node.Marshal(response)
	if marshalErr != nil {
		return marshalErr
	}

	return reply(bytes)
}

// Send 发送消息到集群节点
func (r *Cluster) Send(msg *iface.ActorMessage) (err error) {
	if err = msg.Validate(); err != nil {
		return
	}

	toNodeId := msg.To.GetNodeId()

	if m := r.discovery.GetById(toNodeId); m == nil {
		err = NotFoundMember
		return
	}

	data, mErr := r.node.Marshal(msg)
	if mErr != nil {
		return mErr
	}
	subject := r.makeTopic(toNodeId)

	return r.messageQue.Publish(subject, data)
}

func (r *Cluster) Call(msg *iface.ActorMessage) (bin []byte, err error) {
	if err = msg.Validate(); err != nil {
		return
	}

	toNodeId := msg.To.GetNodeId()

	if m := r.discovery.GetById(toNodeId); m == nil {
		err = NotFoundMember
		return
	}

	data, w := r.node.Marshal(msg.Message)
	if w != nil {
		err = w
		return
	}

	subject := r.makeTopic(toNodeId)
	bytes, w := r.messageQue.Request(subject, data, lib.NowDelay(msg.GetDeadline(), 0))
	if w != nil {
		w = err
		return
	}

	response := &iface.Response{}
	if err = r.node.Unmarshal(bytes, response); err != nil {
		return
	}

	bin = response.GetData()
	err = response.GetError()
	return
}

func (r *Cluster) UpdateMember() error {
	return r.discovery.Add(r.node.Info())
}

func (r *Cluster) Select(service string, strategy discovery.RouteStrategy) uint64 {
	if strategy == nil {
		strategy = discovery.RouteRandom
	}
	// 通过服务发现获取节点列表
	members := r.discovery.GetAll()
	if len(members) == 0 {
		return 0
	}

	var selected []*discovery.Member
	for _, node := range members {
		if !slices.Contains(node.Tags, service) {
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
func (r *Cluster) Broadcast(service string, message *iface.ActorMessage) {
	members := r.discovery.GetAll()
	if len(members) == 0 {
		return
	}
	for _, member := range members {
		if !slices.Contains(member.Tags, service) {
			continue
		}
		msg := convertor.DeepClone(message)
		msg.To = &iface.Pid{
			NodeId: member.GetID(),
			Name:   service,
		}
		// 发送消息
		if err := r.Send(msg); err != nil {
			glog.Error("集群通信: 广播消息到节点失败", zap.Uint64("nodeId", member.GetID()), zap.String("kind", service), zap.Error(err))
		}
	}
	return
}

// Shutdown 关闭所有订阅
func (r *Cluster) Shutdown(ctx context.Context) error {
	if err := r.discovery.Shutdown(ctx); err != nil {
		return err
	}
	if err := r.messageQue.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
