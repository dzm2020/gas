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

func (n *Node) Info() *discovery.Member {
	return n.Member
}
func (n *Node) SetSystem(system iface.ISystem) {
	n.actorSystem = system
}
func (n *Node) System() iface.ISystem {
	return n.actorSystem
}

func (n *Node) SetRemote(remote iface.IRemote) {
	n.remote = remote
}
func (n *Node) Remote() iface.IRemote {
	return n.remote
}

// SetSerializer 设置序列化器
func (n *Node) SetSerializer(ser lib.ISerializer) {
	n.serializer = ser
}

func (n *Node) Serializer() lib.ISerializer {
	return n.serializer
}

func (n *Node) Marshal(request interface{}) []byte {
	// 处理 nil 情况
	if request == nil {
		return []byte{}
	}

	// 如果已经是 []byte 类型，直接返回
	if data, ok := request.([]byte); ok {
		return data
	}

	// 使用序列化器序列化
	if data, err := n.Serializer().Marshal(request); err != nil {
		panic(err)
	} else {
		return data
	}
}

func (n *Node) Unmarshal(data []byte, reply interface{}) {
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
	if err := n.Serializer().Unmarshal(data, reply); err != nil {
		panic(err)
	}
}

func (n *Node) GetConfig() *config.Config {
	return n.config
}

func (n *Node) Startup(comps ...iface.IComponent) error {
	defer lib.PrintCoreDump()
	// 创建节点信息
	n.Member = &discovery.Member{
		Id:      n.config.Node.Id,
		Kind:    n.config.Node.Kind,
		Address: n.config.Node.Address,
		Port:    n.config.Node.Port,
		Tags:    n.config.Node.Tags,
		Meta:    n.config.Node.Meta,
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

	for _, comp := range components {
		if err := n.ComponentManager.Register(comp); err != nil {
			return errs.ErrRegisterComponentFailed(comp.Name(), err)
		}
	}

	// 启动所有组件
	ctx := context.Background()
	if err := n.ComponentManager.Start(ctx); err != nil {
		return errs.ErrStartComponentFailed(err)
	}

	glog.Info("节点启动完成")

	// 阻塞等待进程终止信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	return n.shutdown(context.Background())
}

func (n *Node) SetPanicHook(panicHook func(entry zapcore.Entry)) {
	n.panicHook = panicHook
}

func (n *Node) CallPanicHook(entry zapcore.Entry) {
	if n.panicHook == nil {
		return
	}
	n.panicHook(entry)
}

// Shutdown 优雅关闭节点，关闭所有组件
func (n *Node) shutdown(ctx context.Context) error {
	defer glog.Info("节点停止运行完成")

	glog.Info("节点开始停止运行")

	// 停止所有组件(按逆序停止)
	if err := n.ComponentManager.Stop(ctx); err != nil {
		return err
	}

	n.remote = nil
	n.actorSystem = nil
	n.ComponentManager = nil

	timeoutCtx, _ := context.WithTimeout(ctx, time.Second*30)
	return lib.ShutdownGoroutines(timeoutCtx)
}
