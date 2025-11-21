package node

import (
	"context"
	"fmt"
	"gas/internal/iface"
	"gas/internal/remote"
	"gas/pkg/utils/serializer"
	"time"

	"gas/internal/actor"
	"gas/pkg/component"
	discoveryFactory "gas/pkg/discovery"
	discovery "gas/pkg/discovery/iface"
	messageQueFactory "gas/pkg/messageQue"
	"gas/pkg/utils/glog"
)

type Node struct {
	node        *discovery.Node
	discovery   discovery.IDiscovery
	actorSystem *actor.System
	remote      *remote.Remote
	// 组件管理器
	componentManager *component.Manager
}

func (m *Node) GetId() uint64 {
	if m.node == nil {
		return 0
	}
	return m.node.Id
}

// New 创建节点实例
func New() *Node {
	node := &Node{}
	return node
}

func (m *Node) StarUp(profileFilePath string, comps ...component.Component) error {
	// 读取配置文件
	config, err := loadConfig(profileFilePath)
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}
	// 创建节点信息
	m.node = &discovery.Node{
		Id:      config.Node.Id,
		Name:    config.Node.Name,
		Address: config.Node.Address,
		Port:    config.Node.Port,
		Tags:    config.Node.Tags,
		Meta:    config.Node.Meta,
	}

	// 创建服务发现实例
	discoveryInstance, err := discoveryFactory.NewFromConfig(config.Discovery)
	if err != nil {
		return fmt.Errorf("create discovery failed: %w", err)
	}

	// 创建消息队列实例
	messageQueue, err := messageQueFactory.NewFromConfig(config.MessageQueue)
	if err != nil {
		return fmt.Errorf("create message queue failed: %w", err)
	}

	// 保存 discovery
	m.discovery = discoveryInstance

	// 创建远程通信管理器
	nodeSubjectPrefix := config.Cluster.NodeSubjectPrefix
	m.remote = remote.New(messageQueue, nodeSubjectPrefix, serializer.Json)

	m.componentManager = component.New()

	// 注册组件
	components := []component.Component{
		NewActorComponent("actor", m.node.Id, m),
		NewDiscoveryComponent("discovery", discoveryInstance),
		NewMessageQueComponent("messageQue", messageQueue),
		NewRemoteComponent("remote", m),
	}
	//  注册外部传入的
	components = append(components, comps...)

	for _, c := range components {
		if err := m.componentManager.Register(c); err != nil {
			return fmt.Errorf("register %s component failed: %w", c.Name(), err)
		}
	}

	// 启动所有组件
	ctx := context.Background()
	if err := m.componentManager.Start(ctx); err != nil {
		return fmt.Errorf("start components failed: %w", err)
	}

	// 确保 remote 已设置 actorSystem（组件启动后）
	if m.remote != nil && m.actorSystem != nil {
		m.remote.SetNode(m)
	}

	// 注册节点到服务发现并订阅消息（在组件启动后执行）
	if err := m.registerNode(m.node); err != nil {
		// 如果注册失败，停止已启动的组件
		_ = m.componentManager.Stop(ctx)
		return fmt.Errorf("register node failed: %w", err)
	}

	glog.Infof("node started: id=%d, name=%s, address=%s:%d", m.node.GetID(), m.node.GetName(), m.node.GetAddress(), m.node.GetPort())
	return nil
}

// Self 获取当前节点信息
func (m *Node) Self() *discovery.Node {
	return m.node
}

// GetDiscovery 获取服务发现实例
func (m *Node) GetDiscovery() discovery.IDiscovery {
	return m.discovery
}

func (m *Node) GetSystem() iface.ISystem {
	return m.actorSystem
}
func (m *Node) GetRemote() iface.IRemote {
	return m.remote
}

// registerNode 注册当前节点到服务发现，并自动订阅消息
func (m *Node) registerNode(node *discovery.Node) error {
	// 注册到服务发现
	if err := m.discovery.Add(node); err != nil {
		return err
	}

	// 注册到远程通信管理器
	nodeId := node.GetID()
	if err := m.remote.Subscribe(nodeId); err != nil {
		_ = m.discovery.Remove(nodeId)
		return err
	}

	return nil
}

// unregisterNode 从服务发现注销节点，并移除订阅
func (m *Node) unregisterNode(nodeId uint64) error {
	_ = m.remote.UnregisterNode(nodeId)
	return m.discovery.Remove(nodeId)
}

// Send 发送消息到指定的 actor
func (m *Node) Send(message *iface.Message) error {
	if err := m.validateMessage(message); err != nil {
		return err
	}

	toNodeId := message.GetTo().GetNodeId()
	if m.isLocalNode(toNodeId) {
		if m.actorSystem == nil {
			return fmt.Errorf("actor system not initialized")
		}
		return m.actorSystem.Send(message)
	}

	// 远程发送
	return m.remote.Send(message)
}

// Request 向指定的 actor 发送请求并等待回复
func (m *Node) Request(message *iface.Message, timeout time.Duration) *iface.RespondMessage {
	if err := m.validateMessage(message); err != nil {
		return iface.NewErrorResponse(err.Error())
	}

	toNodeId := message.GetTo().GetNodeId()
	if m.isLocalNode(toNodeId) {
		if m.actorSystem == nil {
			return iface.NewErrorResponse("actor system not initialized")
		}
		return m.actorSystem.Request(message, timeout)
	}

	// 远程调用
	return m.remote.Request(message, timeout)
}

// validateMessage 验证消息有效性
func (m *Node) validateMessage(message *iface.Message) error {
	if message == nil {
		return fmt.Errorf("message is nil")
	}
	if message.GetTo() == nil {
		return fmt.Errorf("target pid is nil")
	}
	if m.Self().GetID() == 0 {
		return fmt.Errorf("self node not registered")
	}
	return nil
}

// isLocalNode 判断是否为本地节点
func (m *Node) isLocalNode(nodeId uint64) bool {
	return nodeId == 0 || nodeId == m.Self().GetID()
}

// Stop 优雅关闭节点，关闭所有组件
func (m *Node) Stop() error {
	// 保存节点信息用于日志
	nodeId := m.Self().GetID()
	nodeName := m.node.GetName()

	if nodeId == 0 {
		return nil
	}

	glog.Infof("node stopping: id=%d, name=%s", nodeId, nodeName)

	// 停止所有组件（按逆序停止：subscription -> messageQue -> discovery -> actor）
	ctx := context.Background()
	if m.componentManager != nil {
		if err := m.componentManager.Stop(ctx); err != nil {
			glog.Errorf("node: stop components failed: %v", err)
		}
	}

	// 清理本地引用
	if m.remote != nil {
		m.remote.Close()
	}
	m.node = nil
	m.discovery = nil
	m.remote = nil
	m.componentManager = nil

	glog.Infof("node stopped: id=%d", nodeId)
	return nil
}
