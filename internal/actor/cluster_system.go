package actor

// Package actor cluster_system.go 提供集群版 Actor 系统（ClusterSystem），在嵌入的 System 基础上
// 通过 transport 支持跨节点消息（Send/Call）
import (
	"fmt"

	"github.com/dzm2020/gas/internal/component/gate/session"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/cluster"
	"github.com/dzm2020/gas/pkg/lib"
)

// NewClusterSystem 创建集群版 Actor 系统。
// selfNodeID 为当前节点 ID，serializer 用于序列化，transport 用于跨节点通信与集群元数据同步。
func NewClusterSystem(selfNodeID uint64, serializer lib.ISerializer, transport cluster.ICluster) *ClusterSystem {
	return &ClusterSystem{
		System:     NewSystem(selfNodeID, serializer),
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

// Send 发送消息：目标为本节点则走 System.Send，否则通过 transport 发往目标节点。
func (s *ClusterSystem) Send(message *iface.ActorMessage) (err error) {
	if s.isLocalMessage(message) {
		return s.System.Send(message)
	}
	data := message.Session.Values[session.KeyMessage]
	fmt.Printf("ClusterSystem Send data:%v \n", []byte(data))
	return s.transport.Send(message.To.NodeId, message)
}

// Call 同步调用：目标为本节点则走 System.Call，否则通过 transport 发往目标节点并等待响应。
func (s *ClusterSystem) Call(message *iface.ActorMessage) (data []byte, err error) {
	timeout := lib.DeadlineToTimeout(message.GetDeadline(), 0)
	if s.isLocalMessage(message) {
		return s.System.Call(message)
	}
	data, err = s.transport.Call(message.To.NodeId, message, timeout)
	if err != nil {
		return nil, err
	}
	//  解析返回值
	response := &iface.Response{}
	if err = s.Serializer().Unmarshal(data, response); err != nil {
		return nil, err
	}
	if response.GetError() != nil {
		return nil, response.GetError()
	}
	return response.GetData(), nil
}
