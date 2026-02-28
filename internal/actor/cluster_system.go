package actor

// Package actor cluster_system.go 提供集群版 Actor 系统（ClusterSystem），在嵌入的 System 基础上
// 通过 transport 支持跨节点消息（Send/Call）与全局命名（Named/Unname）。
import (
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
	*System    // 本地 Actor 系统，负责本节点进程与消息
	selfNodeID uint64
	transport  cluster.ICluster // 集群传输，用于跨节点 Send/Call 与节点信息更新
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
	return s.transport.Send(message.To.NodeId, message)
}

// Call 同步调用：目标为本节点则走 System.Call，否则通过 transport 发往目标节点并等待响应。
func (s *ClusterSystem) Call(message *iface.ActorMessage) (data []byte, err error) {
	timeout := lib.DeadlineToTimeout(message.GetDeadline(), 0)
	if s.isLocalMessage(message) {
		return s.System.Call(message)
	}
	return s.transport.Call(message.To.NodeId, message, timeout)
}

//  tag应该在启动时静态赋值,因为节点的功能在启动时应该是确定的

//// Named 为进程注册名字：先在本地 System 注册，若名字首字母大写则视为全局名并同步到集群（更新节点 Tags）。
//func (s *ClusterSystem) Named(ctx iface.IContext) error {
//	if err := s.System.Named(ctx); err != nil {
//		return err
//	}
//	name := ctx.GetName()
//	isGlobalName := lib.IsFirstLetterUppercase(name)
//	if !isGlobalName {
//		return nil
//	}
//	s.localInfo.Tags = append(s.localInfo.Tags, name)
//	return s.transport.Update(s.localInfo)
//}
//
//// Unname 注销进程名字：先在本地 System 注销，若为全局名则从集群节点 Tags 中移除该名字。
//func (s *ClusterSystem) Unname(ctx iface.IContext) error {
//	if err := s.System.Unname(ctx); err != nil {
//		return err
//	}
//	name := ctx.GetName()
//	isGlobalName := lib.IsFirstLetterUppercase(name)
//	if !isGlobalName {
//		return nil
//	}
//	s.localInfo.Tags = slices.DeleteFunc(s.localInfo.Tags, func(t string) bool {
//		return t == name
//	})
//	return s.transport.Update(s.localInfo)
//}
