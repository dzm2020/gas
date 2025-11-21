package remote

import (
	"fmt"
	"gas/internal/iface"
	messageQue "gas/pkg/messageQue/iface"
	"gas/pkg/utils/glog"
	"gas/pkg/utils/serializer"
	"sync"
	"time"
)

// Remote 远程通信管理器，负责处理跨节点的消息传递
type Remote struct {
	node       iface.INode
	messageQue messageQue.IMessageQue
	serializer serializer.ISerializer
	// 节点主题前缀
	nodeSubjectPrefix string
	// 订阅管理：节点ID -> 订阅对象（每个节点只订阅一次）
	subscriptionsMu sync.RWMutex
	subscriptions   map[uint64]messageQue.ISubscription
}

// New 创建远程通信管理器
func New(que messageQue.IMessageQue, nodeSubjectPrefix string, serializer serializer.ISerializer) *Remote {
	if nodeSubjectPrefix == "" {
		nodeSubjectPrefix = "cluster.node."
	}
	return &Remote{
		messageQue:        que,
		serializer:        serializer,
		nodeSubjectPrefix: nodeSubjectPrefix,
		subscriptions:     make(map[uint64]messageQue.ISubscription),
	}
}

// SetActorSystem 设置 Actor 系统
func (r *Remote) SetNode(node iface.INode) {
	r.node = node
}

// Subscribe 注册节点并订阅消息
func (r *Remote) Subscribe(nodeId uint64) error {
	// 检查是否已经订阅
	r.subscriptionsMu.RLock()
	if _, exists := r.subscriptions[nodeId]; exists {
		r.subscriptionsMu.RUnlock()
		return nil
	}
	r.subscriptionsMu.RUnlock()

	// 订阅消息主题
	handler := func(data []byte, reply func([]byte) error) {
		r.onRemoteHandler(data, reply)
	}
	nodeSubject := fmt.Sprintf("%s%d", r.nodeSubjectPrefix, nodeId)
	subscription, err := r.messageQue.Subscribe(nodeSubject, handler)
	if err != nil {
		return fmt.Errorf("subscribe to node %d failed: %w", nodeId, err)
	}

	// 保存订阅
	r.subscriptionsMu.Lock()
	r.subscriptions[nodeId] = subscription
	r.subscriptionsMu.Unlock()

	glog.Infof("subscribe  nodeSubject %s ", nodeSubject)
	return nil
}

// UnregisterNode 注销节点并移除订阅
func (r *Remote) UnregisterNode(nodeId uint64) error {
	r.subscriptionsMu.Lock()
	defer r.subscriptionsMu.Unlock()

	if sub, exists := r.subscriptions[nodeId]; exists {
		_ = sub.Unsubscribe()
		delete(r.subscriptions, nodeId)
	}

	return nil
}

// onRemoteHandler 处理远程消息
func (r *Remote) onRemoteHandler(data []byte, reply func([]byte) error) {
	message := &iface.Message{}
	if err := r.serializer.Unmarshal(data, message); err != nil {
		r.sendError(reply, fmt.Sprintf("unmarshal message failed: %v", err))
		return
	}
	if r.node == nil {
		r.sendError(reply, "node not initialized")
		return
	}
	if r.node.GetSystem() == nil {
		r.sendError(reply, "actor system not initialized")
		return
	}

	if reply != nil {
		// Request 模式
		response := r.node.GetSystem().Request(message, 5*time.Second)
		responseData, err := r.serializer.Marshal(response)
		if err != nil {
			r.sendError(reply, fmt.Sprintf("marshal response failed: %v", err))
			return
		}
		if err := reply(responseData); err != nil {
			glog.Errorf("remote: send reply failed: %v", err)
		}
	} else {
		// Send 模式
		if err := r.node.GetSystem().Send(message); err != nil {
			glog.Errorf("remote: send message to actor failed: %v", err)
		}
	}
}

// sendError 发送错误响应
func (r *Remote) sendError(reply func([]byte) error, errMsg string) {
	if reply == nil {
		return
	}
	glog.Errorf("remote: %s", errMsg)
	errorResponse := iface.NewErrorResponse(errMsg)
	if errorData, err := r.serializer.Marshal(errorResponse); err == nil {
		_ = reply(errorData)
	}
}

// Send 发送消息到远程节点
func (r *Remote) Send(message *iface.Message) error {
	toNodeId := message.To.GetNodeId()
	data, err := r.serializer.Marshal(message)
	if err != nil {
		return fmt.Errorf("marshal message failed: %w", err)
	}
	subject := fmt.Sprintf("%s%d", r.nodeSubjectPrefix, toNodeId)
	if err := r.messageQue.Publish(subject, data); err != nil {
		return fmt.Errorf("publish to remote node %d failed: %w", toNodeId, err)
	}
	return nil
}

// Request 向远程节点发送请求并等待回复
func (r *Remote) Request(message *iface.Message, timeout time.Duration) *iface.RespondMessage {
	toNodeId := message.To.GetNodeId()
	data, err := r.serializer.Marshal(message)
	if err != nil {
		return iface.NewErrorResponse(fmt.Sprintf("marshal message failed: %v", err))
	}
	subject := fmt.Sprintf("%s%d", r.nodeSubjectPrefix, toNodeId)
	responseData, err := r.messageQue.Request(subject, data, timeout)
	if err != nil {
		return iface.NewErrorResponse(fmt.Sprintf("request to remote node %d failed: %v", toNodeId, err))
	}

	response := &iface.RespondMessage{}
	if err = r.serializer.Unmarshal(responseData, response); err != nil {
		return iface.NewErrorResponse(fmt.Sprintf("unmarshal response failed: %v", err))
	}
	return response
}

// Close 关闭所有订阅
func (r *Remote) Close() {
	r.subscriptionsMu.Lock()
	defer r.subscriptionsMu.Unlock()

	for nodeId, sub := range r.subscriptions {
		_ = sub.Unsubscribe()
		delete(r.subscriptions, nodeId)
	}
}
