package node

import (
	"context"
	"gas/internal/config"
	"gas/internal/errs"
	"gas/internal/iface"
	"gas/pkg/lib"
	"gas/pkg/lib/glog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	discovery "gas/pkg/discovery/iface"
	_ "gas/pkg/discovery/provider/consul"
	_ "gas/pkg/messageQue/provider/nats"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 创建节点实例
func New(path string) *Node {
	node := &Node{
		serializer:       lib.Json,
		ComponentManager: NewComponentsMgr(),
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
		ComponentManager: NewComponentsMgr(),
	}
	return node
}

type Node struct {
	*discovery.Member
	*ComponentManager
	config *config.Config

	actorSystem iface.ISystem
	remote      iface.IRemote

	serializer lib.ISerializer

	panicHook func(entry zapcore.Entry)
}

func (m *Node) Info() *discovery.Member {
	return m.Member
}
func (m *Node) SetSystem(system iface.ISystem) {
	m.actorSystem = system
}
func (m *Node) System() iface.ISystem {
	return m.actorSystem
}

func (m *Node) SetRemote(remote iface.IRemote) {
	m.remote = remote
}
func (m *Node) Remote() iface.IRemote {
	return m.remote
}

// SetSerializer 设置序列化器
func (m *Node) SetSerializer(ser lib.ISerializer) {
	m.serializer = ser
}

func (m *Node) Serializer() lib.ISerializer {
	return m.serializer
}

func (m *Node) Marshal(request interface{}) []byte {
	// 处理 nil 情况
	if request == nil {
		return []byte{}
	}

	// 如果已经是 []byte 类型，直接返回
	if data, ok := request.([]byte); ok {
		return data
	}

	// 使用序列化器序列化
	if data, err := m.Serializer().Marshal(request); err != nil {
		panic(err)
	} else {
		return data
	}
}

func (m *Node) Unmarshal(data []byte, reply interface{}) {
	// 如果数据为空，直接返回
	if len(data) == 0 {
		return
	}

	// 如果目标对象为空，直接返回
	if reply == nil {
		return
	}

	// 如果目标对象是指向 []byte 的指针，直接将数据赋值
	if ptr, ok := reply.(*[]byte); ok {
		*ptr = data
		return
	}

	// 使用序列化器反序列化
	if err := m.Serializer().Unmarshal(data, reply); err != nil {
		panic(err)
	}
}

func (m *Node) GetConfig() *config.Config {
	return m.config
}

func (m *Node) Startup(comps ...iface.IComponent) error {
	defer lib.PrintCoreDump()
	// 创建节点信息
	m.Member = &discovery.Member{
		Id:      m.config.Node.Id,
		Kind:    m.config.Node.Kind,
		Address: m.config.Node.Address,
		Port:    m.config.Node.Port,
		Tags:    m.config.Node.Tags,
		Meta:    m.config.Node.Meta,
	}

	// 注册组件（注意顺序：glog 应该最先初始化，因为其他组件可能会使用日志）
	components := []iface.IComponent{
		NewLogComponent(),
		NewActorComponent(),
		NewRemoteComponent(),
	}

	lib.SetPanicHandler(func(err interface{}) {
		glog.Panic("panic", zap.Any("err", err), zap.String("stack", string(debug.Stack())))
	})

	//  注册外部传入的
	components = append(components, comps...)

	for _, com := range components {
		if err := m.ComponentManager.Register(com); err != nil {
			return errs.ErrRegisterComponentFailed(com.Name(), err)
		}
	}

	// 启动所有组件
	ctx := context.Background()
	if err := m.ComponentManager.Start(ctx); err != nil {
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

func (m *Node) CallPanicHook(entry zapcore.Entry) {
	if m.panicHook == nil {
		return
	}
	m.panicHook(entry)
}

// Shutdown 优雅关闭节点，关闭所有组件
func (m *Node) shutdown(ctx context.Context) error {
	defer glog.Info("节点停止运行完成")

	glog.Info("节点开始停止运行")

	// 停止所有组件(按逆序停止)
	if err := m.ComponentManager.Stop(ctx); err != nil {
		return err
	}

	m.remote = nil
	m.actorSystem = nil
	m.ComponentManager = nil

	timeoutCtx, _ := context.WithTimeout(ctx, time.Second*30)
	return lib.ShutdownGoroutines(timeoutCtx)
}
