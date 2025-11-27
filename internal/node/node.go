package node

import (
	"context"
	"fmt"
	"gas/internal/config"
	"gas/internal/iface"
	"gas/internal/remote"
	"gas/pkg/lib/serializer"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gas/internal/actor"
	"gas/pkg/component"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/lib/glog"
)

// New 创建节点实例
func New() *Node {
	node := &Node{
		serializer: serializer.Json,
	}
	return node
}

type Node struct {
	node *discovery.Node

	config *config.Config

	actorSystem iface.ISystem
	remote      iface.IRemote
	// 组件管理器
	componentManager *component.Manager

	serializer serializer.ISerializer
}

func (m *Node) GetId() uint64 {
	if m.node == nil {
		return 0
	}
	return m.node.Id
}

func (m *Node) SetActorSystem(system iface.ISystem) {
	m.actorSystem = system
}
func (m *Node) GetActorSystem() iface.ISystem {
	return m.actorSystem
}
func (m *Node) SetRemote(remote iface.IRemote) {
	m.remote = remote
}

func (m *Node) GetRemote() iface.IRemote {
	return m.remote
}

// SetSerializer 设置序列化器
func (m *Node) SetSerializer(ser serializer.ISerializer) {
	m.serializer = ser
}

// GetSerializer 获取序列化器
func (m *Node) GetSerializer() serializer.ISerializer {
	return m.serializer
}

func (m *Node) GetConfig() *config.Config {
	return m.config
}

// Self 获取当前节点信息
func (m *Node) Self() *discovery.Node {
	return m.node
}

func (m *Node) StarUp(profileFilePath string, comps ...component.Component) error {
	// 读取配置文件
	c, err := config.Load(profileFilePath)
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}

	m.config = c

	// 创建节点信息
	m.node = &discovery.Node{
		Id:      c.Node.Id,
		Name:    c.Node.Name,
		Address: c.Node.Address,
		Port:    c.Node.Port,
		Tags:    c.Node.Tags,
		Meta:    c.Node.Meta,
	}

	m.componentManager = component.New()

	// 注册组件
	components := []component.Component{
		actor.NewComponent("actorSystem", m),
		remote.NewComponent("remote", m),
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

	glog.Infof("game-node started: id=%d, name=%s, address=%s:%d", m.node.GetID(), m.node.GetName(), m.node.GetAddress(), m.node.GetPort())

	// 阻塞等待进程终止信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	return m.Shutdown()
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
		return fmt.Errorf("self game-node not registered")
	}
	return nil
}

// isLocalNode 判断是否为本地节点
func (m *Node) isLocalNode(nodeId uint64) bool {
	return nodeId == 0 || nodeId == m.Self().GetID()
}

// Shutdown 优雅关闭节点，关闭所有组件
func (m *Node) Shutdown() error {
	// 保存节点信息用于日志
	nodeId := m.Self().GetID()
	nodeName := m.node.GetName()

	if nodeId == 0 {
		return nil
	}

	glog.Infof("game-node stopping: id=%d, name=%s", nodeId, nodeName)

	// 停止所有组件（按逆序停止：subscription -> messageQue -> discovery -> actor）
	ctx := context.Background()
	if m.componentManager != nil {
		if err := m.componentManager.Stop(ctx); err != nil {
			glog.Errorf("game-node: stop components failed: %v", err)
		}
	}

	m.node = nil
	m.remote = nil
	m.actorSystem = nil
	m.componentManager = nil

	glog.Infof("game-node stopped: id=%d", nodeId)
	return nil
}
