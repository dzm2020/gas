package node

import (
	"context"
	"fmt"
	"gas/internal/actor"
	"gas/internal/config"
	"gas/internal/iface"
	"gas/internal/remote"
	"gas/pkg/glog"
	"gas/pkg/lib"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	discovery "gas/pkg/discovery/iface"

	"go.uber.org/zap/zapcore"
)

// New 创建节点实例
func New(profileFilePath string) *Node {
	node := &Node{
		profileFilePath: profileFilePath,
		serializer:      lib.Json,
	}
	return node
}

type Node struct {
	profileFilePath string

	node *discovery.Node

	config *config.Config

	actorSystem iface.ISystem
	remote      iface.IRemote
	// 组件管理器
	componentManager *iface.Manager

	serializer lib.ISerializer

	panicHook func(entry zapcore.Entry)
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
func (m *Node) SetSerializer(ser lib.ISerializer) {
	m.serializer = ser
}

// GetSerializer 获取序列化器
func (m *Node) GetSerializer() lib.ISerializer {
	return m.serializer
}

func (m *Node) GetConfig() *config.Config {
	return m.config
}

// Self 获取当前节点信息
func (m *Node) Self() *discovery.Node {
	return m.node
}

func (m *Node) StarUp(comps ...iface.Component) error {
	// 读取配置文件
	c, err := config.Load(m.profileFilePath)
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

	m.componentManager = iface.New()

	// 注册组件（注意顺序：glog 应该最先初始化，因为其他组件可能会使用日志）
	components := []iface.Component{
		NewGlogComponent(),
		actor.NewComponent(),
		remote.NewComponent(),
	}

	lib.SetPanicHandler(func(err interface{}) {
		glog.Panicf("panic handler: %v stack:%v", err, string(debug.Stack()))
	})

	//  注册外部传入的
	components = append(components, comps...)

	for _, com := range components {
		if err = m.componentManager.Register(com); err != nil {
			return fmt.Errorf("register %s component failed: %w", com.Name(), err)
		}
	}

	// 启动所有组件
	ctx := context.Background()
	if err = m.componentManager.Start(ctx, m); err != nil {
		return fmt.Errorf("start components failed: %w", err)
	}

	glog.Infof("game-node started: id=%d, name=%s, address=%s:%d", m.node.GetID(), m.node.GetName(), m.node.GetAddress(), m.node.GetPort())

	// 阻塞等待进程终止信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	return m.shutdown(context.Background())
}

func (m *Node) SetPanicHook(panicHook func(entry zapcore.Entry)) {
	m.panicHook = panicHook
}

// Shutdown 优雅关闭节点，关闭所有组件
func (m *Node) shutdown(ctx context.Context) error {
	// 保存节点信息用于日志
	nodeId := m.Self().GetID()
	nodeName := m.node.GetName()

	if nodeId == 0 {
		return nil
	}

	glog.Infof("game-node stopping: id=%d, name=%s", nodeId, nodeName)

	// 停止所有组件（按逆序停止：subscription -> messageQue -> discovery -> actor）
	if m.componentManager != nil {
		if err := m.componentManager.Stop(ctx); err != nil {
			glog.Errorf("game-node: stop components failed: %v", err)
		}
	}

	m.node = nil
	m.remote = nil
	m.actorSystem = nil
	m.componentManager = nil

	timeoutCtx, _ := context.WithTimeout(context.Background(), time.Second*30)
	glog.Infof("game-node stopped: id=%d", nodeId)
	return lib.ShutdownGoroutines(timeoutCtx)
}
