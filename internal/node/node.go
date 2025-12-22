package node

import (
	"context"
	"gas/internal/actor"
	"gas/internal/cluster"
	"gas/internal/component"
	"gas/internal/config"
	"gas/internal/iface"
	"gas/pkg/glog"
	"gas/pkg/lib"
	"gas/pkg/lib/xerror"
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
	node := newNode()
	c, err := config.Load(path)
	if err != nil {
		panic(err)
	}
	node.config = c
	return node
}

func NewWithConfig(config *config.Config) *Node {
	node := newNode()
	node.config = config
	return node
}

func newNode() *Node {
	return &Node{
		serializer:        lib.Json,
		IComponentManager: component.NewComponentsMgr(),
	}
}

type Node struct {
	*iface.Member
	iface.IComponentManager
	config *config.Config

	system  iface.ISystem
	cluster iface.ICluster

	serializer lib.ISerializer

	panicHook func(entry zapcore.Entry)
}

func (n *Node) Info() *iface.Member {
	return n.Member
}
func (n *Node) SetSystem(system iface.ISystem) {
	n.system = system
}
func (n *Node) System() iface.ISystem {
	return n.system
}

func (n *Node) SetCluster(cluster iface.ICluster) {
	n.cluster = cluster
}
func (n *Node) Cluster() iface.ICluster {
	return n.cluster
}

// SetSerializer 设置序列化器
func (n *Node) SetSerializer(ser lib.ISerializer) {
	n.serializer = ser
}

func (n *Node) Serializer() lib.ISerializer {
	return n.serializer
}

func (n *Node) Marshal(request interface{}) ([]byte, error) {
	if request == nil {
		return []byte{}, nil
	}

	if data, ok := request.([]byte); ok {
		return data, nil
	}

	return n.Serializer().Marshal(request)
}

func (n *Node) Unmarshal(data []byte, reply interface{}) error {
	if len(data) == 0 {
		return nil
	}

	if reply == nil {
		return nil
	}

	if ptr, ok := reply.(*[]byte); ok {
		*ptr = data
		return nil
	}
	return n.Serializer().Unmarshal(data, reply)
}

func (n *Node) GetConfig() *config.Config {
	return n.config
}

func (n *Node) Startup(comps ...iface.IComponent) error {
	defer xerror.PrintCoreDump()
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
		component.NewLogComponent(),
		actor.NewComponent(),
		cluster.NewComponent(),
	}

	lib.SetPanicHandler(func(err interface{}) {
		glog.Panic("panic", zap.Any("err", err), zap.String("stack", string(debug.Stack())))
	})

	//  注册外部传入的
	components = append(components, comps...)

	for _, comp := range components {
		if err := n.IComponentManager.Register(comp); err != nil {
			return err
		}
	}

	// 启动所有组件
	ctx := context.Background()
	if err := n.IComponentManager.Start(ctx, n); err != nil {
		return err
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
	if err := n.IComponentManager.Stop(ctx); err != nil {
		return err
	}
	shutdownTimeout := 30 * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()
	return lib.ShutdownGoroutines(timeoutCtx)
}
