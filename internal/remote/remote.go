package remote

import (
	"context"
	"fmt"
	"gas/internal/errs"
	"gas/internal/iface"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/lib"
	"gas/pkg/lib/glog"
	messageQue "gas/pkg/messageQue/iface"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

// Remote 远程通信管理器，负责处理跨节点的消息传递
type Remote struct {
	node iface.INode

	serializer lib.ISerializer

	messageQue messageQue.IMessageQue

	// 节点主题前缀
	nodeSubjectPrefix string

	discovery discovery.IDiscovery
}

func (r *Remote) Start(ctx context.Context) error {
	if err := r.messageQue.Run(ctx); err != nil {
		return err
	}
	if err := r.discovery.Run(ctx); err != nil {
		return err
	}
	// 加入节点
	if err := r.discovery.Add(r.node.Info()); err != nil {
		return err
	}
	//  监听集群消息
	if err := r.listen(); err != nil {
		return err
	}
	return nil
}

// SetSerializer 设置序列化器
func (r *Remote) SetSerializer(ser lib.ISerializer) {
	r.serializer = ser
}
func (r *Remote) SetNode(node iface.INode) {
	r.node = node
}

func (r *Remote) listen() error {
	nodeId := r.node.GetID()
	// 订阅消息主题
	handler := func(data []byte, reply func([]byte) error) {
		r.onRemoteHandler(data, reply)
	}
	nodeSubject := fmt.Sprintf("%s%d", r.nodeSubjectPrefix, nodeId)
	_, err := r.messageQue.Subscribe(nodeSubject, handler)
	if err != nil {
		return err
	}
	return nil
}

// onRemoteHandler 处理远程消息
func (r *Remote) onRemoteHandler(data []byte, reply func([]byte) error) {
	message := &iface.Message{}
	if err := r.serializer.Unmarshal(data, message); err != nil {
		r.sendError(reply, err)
		return
	}
	if r.node == nil {
		r.sendError(reply, errs.ErrNodeIsNil)
		return
	}
	if r.node.GetActorSystem() == nil {
		r.sendError(reply, errs.ErrActorSystemIsNil)
		return
	}

	if reply != nil {
		response := r.node.GetActorSystem().Call(message, 5*time.Second)
		responseData, err := r.serializer.Marshal(response)
		if err != nil {
			r.sendError(reply, err)
			return
		}
		if err = reply(responseData); err != nil {
			glog.Error("远程通信: 发送回复失败", zap.Error(err))
		}
	} else {
		// Send 模式
		if err := r.node.GetActorSystem().Send(message); err != nil {
			glog.Error("远程通信: 发送消息到 Actor 失败", zap.Error(err))
		}
	}
	glog.Debug("远程通信: 发送消息到 Actor", zap.Any("message", message), zap.Int64("msgId", message.GetId()))
}

// sendError 发送错误响应
func (r *Remote) sendError(reply func([]byte) error, err error) {
	if reply == nil {
		return
	}
	glog.Error("远程通信: 发送错误响应", zap.Error(err))
	errorResponse := iface.NewErrorResponse(err)
	if errorData, err := r.serializer.Marshal(errorResponse); err == nil {
		_ = reply(errorData)
	}
}

// Send 发送消息到远程节点
func (r *Remote) Send(message *iface.Message) error {
	toNodeId := message.To.GetNodeId()
	data, err := r.serializer.Marshal(message)
	if err != nil {
		return errs.ErrMarshalMessageFailed(err)
	}
	subject := fmt.Sprintf("%s%d", r.nodeSubjectPrefix, toNodeId)
	if err = r.messageQue.Publish(subject, data); err != nil {
		return errs.ErrPublishToRemoteNodeFailed(toNodeId, err)
	}
	return nil
}

// Call 向远程节点发送请求并等待回复
func (r *Remote) Call(message *iface.Message, timeout time.Duration) *iface.Response {
	toNodeId := message.To.GetNodeId()
	data, err := r.serializer.Marshal(message)
	if err != nil {
		return iface.NewErrorResponse(err)
	}
	subject := fmt.Sprintf("%s%d", r.nodeSubjectPrefix, toNodeId)
	responseData, err := r.messageQue.Request(subject, data, timeout)
	if err != nil {
		return iface.NewErrorResponse(err)
	}

	response := &iface.Response{}
	if err = r.serializer.Unmarshal(responseData, response); err != nil {
		return iface.NewErrorResponse(err)
	}
	return response
}

func (r *Remote) UpdateNode() error {
	return r.discovery.Add(r.node.Info())
}

func (r *Remote) Select(service string, strategy iface.RouteStrategy) *iface.Pid {
	if strategy == nil {
		strategy = RouteRandom
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
func (r *Remote) Broadcast(service string, message *iface.Message) {
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
