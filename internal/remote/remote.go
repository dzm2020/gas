package remote

import (
	"fmt"
	"gas/internal/iface"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/lib/glog"
	"gas/pkg/lib/serializer"
	messageQue "gas/pkg/messageQue/iface"
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

	discovery discovery.IDiscovery
}

// New 创建远程通信管理器
func New(que messageQue.IMessageQue, discovery discovery.IDiscovery, nodeSubjectPrefix string, serializer serializer.ISerializer) *Remote {
	if nodeSubjectPrefix == "" {
		nodeSubjectPrefix = "cluster.game-node."
	}
	return &Remote{
		discovery:         discovery,
		messageQue:        que,
		serializer:        serializer,
		nodeSubjectPrefix: nodeSubjectPrefix,
		subscriptions:     make(map[uint64]messageQue.ISubscription),
	}
}

// SetSerializer 设置序列化器
func (r *Remote) SetSerializer(ser serializer.ISerializer) {
	r.serializer = ser
}

func (r *Remote) registry(node *discovery.Node) error {
	// 注册到服务发现
	if err := r.discovery.Add(node); err != nil {
		return err
	}

	// 注册到远程通信管理器
	nodeId := node.GetID()
	if err := r.Subscribe(nodeId); err != nil {
		_ = r.discovery.Remove(nodeId)
		return err
	}
	return nil
}

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
		return fmt.Errorf("subscribe to game-node %d failed: %w", nodeId, err)
	}

	// 保存订阅
	r.subscriptionsMu.Lock()
	r.subscriptions[nodeId] = subscription
	r.subscriptionsMu.Unlock()

	glog.Infof("subscribe  nodeSubject %s ", nodeSubject)
	return nil
}

func (r *Remote) Unsubscribe(nodeId uint64) error {
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
		r.sendError(reply, "game-node not initialized")
		return
	}
	if r.node.GetActorSystem() == nil {
		r.sendError(reply, "actor system not initialized")
		return
	}

	if reply != nil {
		// Request 模式
		response := r.node.GetActorSystem().Request(message, 5*time.Second)
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
		if err := r.node.GetActorSystem().Send(message); err != nil {
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
		return fmt.Errorf("publish to remote game-node %d failed: %w", toNodeId, err)
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
		return iface.NewErrorResponse(fmt.Sprintf("request to remote game-node %d failed: %v", toNodeId, err))
	}

	response := &iface.RespondMessage{}
	if err = r.serializer.Unmarshal(responseData, response); err != nil {
		return iface.NewErrorResponse(fmt.Sprintf("unmarshal response failed: %v", err))
	}
	return response
}

// SendByService 通过服务名发送消息到远程节点
func (r *Remote) SendByService(service string, message *iface.Message, strategy RouteStrategy) error {
	if strategy == nil {
		strategy = RouteRandom
	}

	// 通过服务发现获取节点列表
	nodes := r.discovery.GetService(service)
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found for service: %s", service)
	}

	// 使用路由策略选择节点
	selectedNode := strategy(nodes)
	if selectedNode == nil {
		return fmt.Errorf("route strategy returned nil node for service: %s", service)
	}

	// 确保已订阅该节点
	if err := r.Subscribe(selectedNode.GetID()); err != nil {
		return fmt.Errorf("subscribe to node %d failed: %w", selectedNode.GetID(), err)
	}

	// 设置消息的目标节点ID
	if message.To == nil {
		message.To = &iface.Pid{}
	}
	message.To.NodeId = selectedNode.GetID()

	// 发送消息
	return r.Send(message)
}

// RequestByService 通过服务名向远程节点发送请求并等待回复
func (r *Remote) RequestByService(service string, message *iface.Message, timeout time.Duration, strategy RouteStrategy) *iface.RespondMessage {
	if strategy == nil {
		strategy = RouteRandom
	}

	// 通过服务发现获取节点列表
	nodes := r.discovery.GetService(service)
	if len(nodes) == 0 {
		return iface.NewErrorResponse(fmt.Sprintf("no nodes found for service: %s", service))
	}

	// 使用路由策略选择节点
	selectedNode := strategy(nodes)
	if selectedNode == nil {
		return iface.NewErrorResponse(fmt.Sprintf("route strategy returned nil node for service: %s", service))
	}

	// 确保已订阅该节点
	if err := r.Subscribe(selectedNode.GetID()); err != nil {
		return iface.NewErrorResponse(fmt.Sprintf("subscribe to node %d failed: %v", selectedNode.GetID(), err))
	}

	// 设置消息的目标节点ID
	if message.To == nil {
		message.To = &iface.Pid{}
	}
	message.To.NodeId = selectedNode.GetID()

	// 发送请求
	return r.Request(message, timeout)
}

// Broadcast 向服务的所有节点广播消息
func (r *Remote) Broadcast(service string, message *iface.Message) error {
	// 通过服务发现获取节点列表
	nodes := r.discovery.GetService(service)
	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found for service: %s", service)
	}

	var lastErr error
	// 向所有节点发送消息
	for _, node := range nodes {
		// 确保已订阅该节点
		if err := r.Subscribe(node.GetID()); err != nil {
			glog.Errorf("subscribe to node %d failed: %v", node.GetID(), err)
			lastErr = err
			continue
		}

		// 为每个节点创建消息副本并设置目标节点ID
		nodeMessage := &iface.Message{
			To: &iface.Pid{
				NodeId: node.GetID(),
			},
			From: message.From,
			Id:   message.Id,
			Data: message.Data,
		}
		// 如果原消息有 To 字段，保留 ServiceId 和 Name
		if message.To != nil {
			nodeMessage.To.ServiceId = message.To.GetServiceId()
			nodeMessage.To.Name = message.To.GetName()
		}

		// 发送消息
		if err := r.Send(nodeMessage); err != nil {
			glog.Errorf("broadcast to node %d failed: %v", node.GetID(), err)
			lastErr = err
		}
	}

	return lastErr
}

// Shutdown 关闭所有订阅
func (r *Remote) Shutdown() error {
	r.subscriptionsMu.Lock()
	defer r.subscriptionsMu.Unlock()
	//  取消订阅
	for nodeId, sub := range r.subscriptions {
		_ = sub.Unsubscribe()
		delete(r.subscriptions, nodeId)
	}
	nodeId := r.node.GetId()

	if err := r.discovery.Remove(nodeId); err != nil {
		return err
	}

	//  注销节点
	if err := r.discovery.Close(); err != nil {
		return err
	}
	if err := r.messageQue.Close(); err != nil {
		return err
	}
	return nil
}
