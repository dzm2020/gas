package remote

import (
	"fmt"
	"gas/internal/iface"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/glog"
	"gas/pkg/lib"
	messageQue "gas/pkg/messageQue/iface"
	"time"

	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

// Remote 远程通信管理器，负责处理跨节点的消息传递
type Remote struct {
	node       iface.INode
	messageQue messageQue.IMessageQue
	serializer lib.ISerializer
	// 节点主题前缀
	nodeSubjectPrefix string

	discovery discovery.IDiscovery
}

// SetSerializer 设置序列化器
func (r *Remote) SetSerializer(ser lib.ISerializer) {
	r.serializer = ser
}

func (r *Remote) init() error {
	node := r.node.Self()
	// 注册到服务发现
	if err := r.discovery.Add(node); err != nil {
		return err
	}

	// 注册到远程通信管理器
	nodeId := node.GetID()
	if err := r.subscribe(nodeId); err != nil {
		_ = r.discovery.Remove(nodeId)
		return err
	}
	return nil
}

func (r *Remote) SetNode(node iface.INode) {
	r.node = node
}

// Subscribe 注册节点并订阅消息
func (r *Remote) subscribe(nodeId uint64) error {
	// 订阅消息主题
	handler := func(data []byte, reply func([]byte) error) {
		r.onRemoteHandler(data, reply)
	}
	nodeSubject := fmt.Sprintf("%s%d", r.nodeSubjectPrefix, nodeId)
	_, err := r.messageQue.Subscribe(nodeSubject, handler)
	if err != nil {
		return fmt.Errorf("subscribe to game-node %d failed: %w", nodeId, err)
	}

	glog.Infof("subscribe  nodeSubject %s ", nodeSubject)
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
		response := r.node.GetActorSystem().Call(message, 5*time.Second)
		responseData, err := r.serializer.Marshal(response)
		if err != nil {
			r.sendError(reply, fmt.Sprintf("marshal response failed: %v", err))
			return
		}
		if err = reply(responseData); err != nil {
			glog.Errorf("remote: send reply failed: %v", err)
		}
	} else {
		// Send 模式
		if err := r.node.GetActorSystem().Send(message); err != nil {
			glog.Errorf("remote: send message to actor failed: %v", err)
		}
	}
	glog.Info("remote: send message to actor", zap.Any("message", message))
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
func (r *Remote) Request(message *iface.Message, timeout time.Duration) *iface.Response {
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

	response := &iface.Response{}
	if err = r.serializer.Unmarshal(responseData, response); err != nil {
		return iface.NewErrorResponse(fmt.Sprintf("unmarshal response failed: %v", err))
	}
	return response
}
func (r *Remote) RegistryName(name string) error {
	info := r.node.Self()
	if slices.Contains(info.GetTags(), name) {
		return nil
	}
	info.Tags = append(info.Tags, name)
	return r.discovery.Add(info)
}

func (r *Remote) Select(service string, strategy iface.RouteStrategy) (*iface.Pid, error) {
	if strategy == nil {
		strategy = RouteRandom
	}

	// 通过服务发现获取节点列表
	nodes := r.discovery.GetAll()
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes found for service: %s", service)
	}

	var _nodes []*discovery.Node
	for _, node := range nodes {
		if !slices.Contains(node.Tags, service) {
			continue
		}
		_nodes = append(_nodes, node)
	}

	// 使用路由策略选择节点
	selectedNode := strategy(_nodes)
	if selectedNode == nil {
		return nil, fmt.Errorf("route strategy returned nil node for service: %s", service)
	}

	// 设置消息的目标节点ID
	return &iface.Pid{
		NodeId: selectedNode.GetID(),
		Name:   service,
	}, nil
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
		// 为每个节点创建消息副本并设置目标节点ID
		nodeMessage := &iface.Message{
			To: &iface.Pid{
				NodeId: node.GetID(),
				Name:   service,
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
	if err := r.discovery.Close(); err != nil {
		return err
	}
	if err := r.messageQue.Close(); err != nil {
		return err
	}
	return nil
}
