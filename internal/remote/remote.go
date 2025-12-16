package remote

import (
	"context"
	"fmt"
	"gas/internal/iface"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/lib/glog"
	messageQue "gas/pkg/messageQue/iface"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

func New(discovery discovery.IDiscovery, messageQue messageQue.IMessageQue, subjectPrefix string) *Remote {
	return &Remote{
		discovery:         discovery,
		messageQue:        messageQue,
		nodeSubjectPrefix: subjectPrefix,
	}
}

// Remote 远程通信管理器，负责处理跨节点的消息传递
type Remote struct {
	discovery         discovery.IDiscovery
	messageQue        messageQue.IMessageQue
	nodeSubjectPrefix string
}

func (r *Remote) Start(ctx context.Context) error {
	if err := r.messageQue.Run(ctx); err != nil {
		return err
	}
	if err := r.discovery.Run(ctx); err != nil {
		return err
	}
	if err := r.discovery.Add(iface.GetNode().Info()); err != nil {
		return err
	}
	if err := r.listen(); err != nil {
		return err
	}
	return nil
}

func (r *Remote) makeTopic(nodeId uint64) string {
	return fmt.Sprintf("%s%d", r.nodeSubjectPrefix, nodeId)
}

func (r *Remote) listen() error {
	nodeId := iface.GetNode().GetID()
	handler := func(data []byte, reply func([]byte) error) {
		if err := r.onRemoteProcess(data, reply); err != nil {
			glog.Error("远程通信: 处理远程消息失败", zap.Error(err))
		}
	}
	subject := r.makeTopic(nodeId)
	_, err := r.messageQue.Subscribe(subject, handler)
	return err
}

// onRemoteProcess 处理远程消息
func (r *Remote) onRemoteProcess(data []byte, reply func([]byte) error) error {
	message := &iface.Message{}
	iface.GetNode().Unmarshal(data, message)

	glog.Debug("远程通信: 处理远程消息", zap.Any("message", message))

	msg := &iface.ActorMessage{Message: message}
	if reply != nil {
		response := iface.GetNode().System().Call(msg, 5*time.Second)
		responseData := iface.GetNode().Marshal(response)
		return reply(responseData)
	} else {
		return iface.GetNode().System().Send(msg)
	}
}

// sendError 发送错误响应
func (r *Remote) sendError(reply func([]byte) error, err error) {
	if reply == nil {
		return
	}
	glog.Error("远程通信: 发送错误响应", zap.Error(err))
	errorResponse := iface.NewErrorResponse(err)
	_ = reply(iface.GetNode().Marshal(errorResponse))
}

// Send 发送消息到远程节点
func (r *Remote) Send(rpcMessage *iface.ActorMessage) error {
	rpcMessage.Async = true
	if err := rpcMessage.Validate(); err != nil {
		return err
	}
	toNodeId := rpcMessage.To.GetNodeId()
	data := iface.GetNode().Marshal(rpcMessage)
	subject := r.makeTopic(toNodeId)
	return r.messageQue.Publish(subject, data)
}

// Call 向远程节点发送请求并等待回复
func (r *Remote) Call(rpcMessage *iface.ActorMessage, timeout time.Duration) *iface.Response {
	rpcMessage.Async = false
	if err := rpcMessage.Validate(); err != nil {
		return iface.NewErrorResponse(err)
	}
	toNodeId := rpcMessage.To.GetNodeId()
	data := iface.GetNode().Marshal(rpcMessage.Message)
	subject := r.makeTopic(toNodeId)
	responseData, err := r.messageQue.Request(subject, data, timeout)
	if err != nil {
		return iface.NewErrorResponse(err)
	}
	response := &iface.Response{}
	iface.GetNode().Unmarshal(responseData, response)
	return response
}

func (r *Remote) UpdateNode() error {
	return r.discovery.Add(iface.GetNode().Info())
}

func (r *Remote) Select(service string, strategy iface.RouteStrategy) *iface.Pid {
	if strategy == nil {
		strategy = iface.RouteRandom
	}

	// 通过服务发现获取节点列表
	members := r.discovery.GetAll()
	if len(members) == 0 {
		return nil
	}

	var _member []*discovery.Member
	for _, node := range members {
		if !slices.Contains(node.Tags, service) {
			continue
		}
		_member = append(_member, node)
	}

	// 使用路由策略选择节点
	selectedNode := strategy(_member)
	if selectedNode == nil {
		return nil
	}

	// 设置消息的目标节点ID
	return &iface.Pid{
		NodeId: selectedNode.GetID(),
		Name:   service,
	}
}

// Broadcast 向服务的所有节点广播消息
func (r *Remote) Broadcast(service string, message *iface.ActorMessage) {
	members := r.discovery.GetAll()
	if len(members) == 0 {
		return
	}
	// 向所有节点发送消息
	for _, member := range members {
		if !slices.Contains(member.Tags, service) {
			continue
		}
		rpcMessage := convertor.DeepClone(message)
		rpcMessage.To = &iface.Pid{
			NodeId: member.GetID(),
			Name:   service,
		}
		// 发送消息
		if err := r.Send(rpcMessage); err != nil {
			glog.Error("远程通信: 广播消息到节点失败", zap.Uint64("nodeId", member.GetID()), zap.String("kind", service), zap.Error(err))
		}
	}
	return
}

// Shutdown 关闭所有订阅
func (r *Remote) Shutdown(ctx context.Context) error {
	if err := r.discovery.Shutdown(ctx); err != nil {
		return err
	}
	if err := r.messageQue.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
