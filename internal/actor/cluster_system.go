package actor

// Package actor cluster_system.go 提供集群版 Actor 系统（ClusterSystem），在嵌入的 System 基础上
// 通过 transport 支持跨节点消息（SendMessage/CallMessage、Send/Call），并挂载可 PublishCluster 的事件总线、订阅 gas.event.>。
import (
	"errors"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/dzm2020/gas/api/pb"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/cluster"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/serializer"
	"github.com/dzm2020/gas/pkg/lib/timer"
	messageQue "github.com/dzm2020/gas/pkg/messageQue/iface"
	"go.uber.org/zap"
)

// eventSubscriber
// @Description: 集群事件回调
type eventSubscriber struct {
	system *System
}

// OnMessage 实现 messageQue.ISubscriber：将 MQ 消息解码后对本机 eventBus 做 PublishLocal。
func (c *eventSubscriber) OnMessage(data []byte, _ func([]byte) error) {
	env := &pb.EventEnvelope{}
	if err := c.system.Serializer().Unmarshal(data, env); err != nil {
		glog.Error("集群：处理事件订阅消息失败", zap.Error(err))
		return
	}
	if env.Topic == "" {
		return
	}
	c.system.PublishLocal(env.Topic, env.Payload)
}

// messageSubscriber
// @Description: 集群actor消息回调
type messageSubscriber struct {
	system *System
}

func (r *messageSubscriber) OnMessage(data []byte, response func(data []byte) error) {
	msg := &iface.ActorMessage{Message: &pb.Message{}}

	ser := r.system.Serializer()
	if err := ser.Unmarshal(data, msg.Message); err != nil {
		glog.Error("集群：处理消息失败", zap.Error(err))
		return
	}
	system := r.system
	if msg.GetAsync() {
		if err := system.SendMessage(msg); err != nil {
			glog.Error("集群：处理消息失败", zap.Error(err))
			return
		}
		return
	}
	//  调用本地 actor（传输层已有完整 Message，使用 CallMessage）
	responseData, responseErr := system.CallMessage(msg)
	//  打包结果
	responseMessage := iface.NewResponse(responseData, responseErr)
	responseData, err := ser.Marshal(responseMessage)
	if err != nil {
		glog.Error("集群：处理消息失败", zap.Error(err))
		return
	}
	//  写入到消息队列
	if err := response(responseData); err != nil {
		glog.Error("集群：处理消息失败", zap.Error(err))
		return
	}
	glog.Debug("集群：处理消息", zap.Any("message", msg))
}

var (
	_ iface.IEventBus = (*ClusterSystem)(nil)
	_ iface.ISystem   = (*ClusterSystem)(nil)
)

// NewClusterSystem 创建集群版 Actor 系统：事件可 PublishCluster；启动时订阅 gas.event.> 并解码投递本地订阅者。
func NewClusterSystem(selfNodeID uint64, ser serializer.ISerializer, transport cluster.ICluster) (*ClusterSystem, error) {
	if transport == nil {
		return nil, errors.New("actor: cluster transport 不能为空")
	}
	sys := newSystem(selfNodeID, ser)
	//  订阅事件
	sub, err := transport.Subscribe(EventNATSSubscribePattern, &eventSubscriber{system: sys})
	if err != nil {
		return nil, err
	}

	//  消息订阅
	if _, err = transport.Subscribe(convertor.ToString(selfNodeID), &messageSubscriber{system: sys}); err != nil {
		_ = sub.Unsubscribe()
		return nil, err
	}

	return &ClusterSystem{
		System:     sys,
		transport:  transport,
		selfNodeID: selfNodeID,
		eventSub:   sub,
	}, nil
}

// ClusterSystem 在 System 之上增加集群能力：本地消息走嵌入的 System，跨节点走 transport。
// 事件订阅与 PublishLocal 使用嵌入 *System 的 IEventBus；PublishCluster 由本类型实现。
type ClusterSystem struct {
	*System                     // 本地 Actor 系统，负责本节点进程与消息
	selfNodeID uint64           // 本节点ID
	transport  cluster.ICluster // 集群传输，用于跨节点 Send/Call 与事件 MQ
	eventSub   messageQue.ISubscription
}

func (s *ClusterSystem) Spawn(actor iface.IActor, args ...interface{}) *iface.Pid {
	return spawn(s, actor, args...)
}

// EventBus 返回本节点完整事件接口（Subscribe/PublishLocal 来自 *System，PublishCluster 为集群实现）。
func (s *ClusterSystem) EventBus() iface.IEventBus {
	return s
}

// Shutdown 先取消集群事件 MQ 订阅，再关闭本地 Actor 系统。
func (s *ClusterSystem) Shutdown() error {
	if s.eventSub != nil {
		_ = s.eventSub.Unsubscribe()
		s.eventSub = nil
	}
	return s.System.Shutdown()
}

// isLocalMessage 判断消息目标是否为本节点（按 NodeId 比较）。
func (s *ClusterSystem) isLocalMessage(message *iface.ActorMessage) bool {
	return message.GetTo().GetNodeId() == s.NodeId()
}

// SendMessage 发送已构造的 ActorMessage：本节点走 System.SendMessage，跨节点走 transport。
func (s *ClusterSystem) SendMessage(message *iface.ActorMessage) (err error) {
	if s.isLocalMessage(message) {
		return s.System.SendMessage(message)
	}
	return s.transport.Send(message.GetTo().GetNodeId(), message)
}

// CallMessage 同步发送已构造的 ActorMessage：本节点走 System.CallMessage，跨节点走 transport。
func (s *ClusterSystem) CallMessage(message *iface.ActorMessage) (data []byte, err error) {
	timeout := timer.DeadlineToTimeout(message.GetDeadline(), 0)
	if s.isLocalMessage(message) {
		return s.System.CallMessage(message)
	}
	data, err = s.transport.Call(message.GetTo().GetNodeId(), message, timeout)
	if err != nil {
		return nil, err
	}
	response := &iface.Response{}
	if err = s.Serializer().Unmarshal(data, response); err != nil {
		return nil, err
	}
	if response.GetError() != nil {
		return nil, response.GetError()
	}
	return response.GetData(), nil
}

// Send 便捷版：构造消息后发送，与 IContext.Send 参数形式一致。
func (s *ClusterSystem) Send(from, to *iface.Pid, methodName string, request interface{}) error {
	bin, err := s.Serializer().Marshal(request)
	if err != nil {
		return err
	}
	message := iface.NewActorMessage(from, to, methodName, bin)
	message.Async = true
	return s.SendMessage(message)
}

// Call 便捷版：构造消息后同步调用，响应反序列化到 reply。超时由 SetCallTimeout 设置。
func (s *ClusterSystem) Call(from, to *iface.Pid, methodName string, request interface{}, reply interface{}, timeout time.Duration) error {
	bin, err := s.Serializer().Marshal(request)
	if err != nil {
		return err
	}
	message := iface.NewActorMessage(from, to, methodName, bin)
	message.Async = false
	message.Deadline = time.Now().Add(timeout).Unix()
	data, err := s.CallMessage(message)
	if err != nil {
		return err
	}
	return s.Serializer().Unmarshal(data, reply)
}

func (s *ClusterSystem) PublishCluster(topic string, payload []byte) error {
	if topic == "" {
		return ErrEventTopicEmpty
	}
	if s.transport == nil {
		return ErrEventNoCluster
	}

	env := &pb.EventEnvelope{
		Topic:      topic,
		Payload:    payload,
		SourceNode: s.selfNodeID,
	}
	data, err := s.Serializer().Marshal(env)
	if err != nil {
		return err
	}
	return s.transport.PublishSubject(EventNATSSubjectPrefix+topic, data)
}
