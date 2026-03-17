package actor

// Package actor cluster_system.go 提供集群版 Actor 系统（ClusterSystem），在嵌入的 System 基础上
// 通过 transport 支持跨节点消息（SendMessage/CallMessage、Send/Call）。
import (
	"time"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/cluster"
	"github.com/dzm2020/gas/pkg/lib/serializer"
	"github.com/dzm2020/gas/pkg/lib/timer"
)

// NewClusterSystem 创建集群版 Actor 系统。
// selfNodeID 为当前节点 ID，ser 用于序列化，transport 用于跨节点通信与集群元数据同步。
func NewClusterSystem(selfNodeID uint64, ser serializer.ISerializer, transport cluster.ICluster) *ClusterSystem {
	return &ClusterSystem{
		System:     NewSystem(selfNodeID, ser),
		transport:  transport,
		selfNodeID: selfNodeID,
	}
}

// ClusterSystem 在 System 之上增加集群能力：本地消息走嵌入的 System，跨节点走 transport。
type ClusterSystem struct {
	*System                     // 本地 Actor 系统，负责本节点进程与消息
	selfNodeID uint64           // 本节点ID
	transport  cluster.ICluster // 集群传输，用于跨节点 Send/Call
}

func (s *ClusterSystem) Spawn(actor iface.IActor, args ...interface{}) *iface.Pid {
	return spawn(s, actor, args...)
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
