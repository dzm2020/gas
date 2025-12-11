package node

import (
	"context"
	"gas/internal/actor"
	"gas/internal/config"
	"gas/internal/errs"
	"gas/internal/iface"
	"gas/internal/remote"
	"gas/pkg/lib"
	"gas/pkg/lib/glog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	discovery "gas/pkg/discovery/iface"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 创建节点实例
func New(path string) *Node {
	node := &Node{
		serializer:       lib.Json,
		componentManager: NewComponentsMgr(),
	}
	c, err := config.Load(path)
	if err != nil {
		panic(err)
	}
	node.config = c
	return node
}

func NewWithConfig(config *config.Config) *Node {
	node := &Node{
		config:           config,
		serializer:       lib.Json,
		componentManager: NewComponentsMgr(),
	}
	return node
}

type Node struct {
	*discovery.Node

	config *config.Config

	actorSystem iface.ISystem
	remote      iface.IRemote
	// 组件管理器
	componentManager *Manager

	serializer lib.ISerializer

	panicHook func(entry zapcore.Entry)
}

func (m *Node) Info() *discovery.Node {
	return m.Node
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

func (m *Node) StarUp(comps ...iface.IComponent) error {
	defer lib.PrintCoreDump()
	// 创建节点信息
	m.Node = &discovery.Node{
		Id:      m.config.Node.Id,
		Name:    m.config.Node.Name,
		Address: m.config.Node.Address,
		Port:    m.config.Node.Port,
		Tags:    m.config.Node.Tags,
		Meta:    m.config.Node.Meta,
	}

	// 注册组件（注意顺序：glog 应该最先初始化，因为其他组件可能会使用日志）
	components := []iface.IComponent{
		NewLogComponent(),
		actor.NewComponent(),
		remote.NewComponent(),
	}

	lib.SetPanicHandler(func(err interface{}) {
		glog.Panic("panic", zap.Any("err", err), zap.String("stack", string(debug.Stack())))
	})

	//  注册外部传入的
	components = append(components, comps...)

	for _, com := range components {
		if err := m.componentManager.Register(com); err != nil {
			return errs.ErrRegisterComponentFailed(com.Name(), err)
		}
	}

	// 启动所有组件
	ctx := context.Background()
	if err := m.componentManager.Start(ctx, m); err != nil {
		return errs.ErrStartComponentFailed(err)
	}

	glog.Info("节点启动完成")

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

	defer glog.Info("节点停止运行完成")

	// 保存节点信息用于日志

	glog.Info("节点开始停止运行")

	// 停止所有组件(按逆序停止)
	if m.componentManager != nil {
		if err := m.componentManager.Stop(ctx); err != nil {
			return err
		}
	}

	m.remote = nil
	m.actorSystem = nil
	m.componentManager = nil

	timeoutCtx, _ := context.WithTimeout(context.Background(), time.Second*30)
	return lib.ShutdownGoroutines(timeoutCtx)
}
