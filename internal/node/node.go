package node

import (
	"context"
	"gas/internal/actor"
	"gas/internal/cluster"
	"gas/internal/iface"
	"gas/internal/logger"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/glog"
	"gas/pkg/lib"
	"gas/pkg/lib/component"
	"gas/pkg/lib/xerror"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	_ "gas/pkg/discovery/provider/consul"
	_ "gas/pkg/messageQue/provider/nats"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/spf13/viper"
)

// New 创建节点实例
func New(path string) *Node {
	node := &Node{
		Member:     new(discovery.Member),
		serializer: lib.Json,
		IManager:   component.NewComponentsMgr[iface.INode](),
		path:       path,
		viper:      viper.New(),
	}
	node.viper.SetConfigType("yaml")
	node.viper.SetConfigFile(path)
	return node
}

type Node struct {
	*iface.Member
	component.IManager[iface.INode]
	path string

	system  iface.ISystem
	cluster iface.ICluster

	serializer lib.ISerializer

	panicHook func(entry zapcore.Entry)

	viper *viper.Viper
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

func (n *Node) GetConfig(key string, cfg interface{}) error {
	return n.viper.UnmarshalKey(key, cfg)
}

func (n *Node) SetDefaultConfig(key string, cfg interface{}) {
	n.viper.SetDefault(key, cfg)
}

func (n *Node) init() {
	n.SetDefaultConfig("node", new(discovery.Member))
}

func (n *Node) Startup(comps ...component.IComponent[iface.INode]) error {
	defer xerror.PrintCoreDump()
	var err error
	//  读取配置内容
	err = n.viper.ReadInConfig() // 读取配置文件
	if err != nil {
		return err
	}
	if err = n.GetConfig("node", n.Member); err != nil {
		return err
	}

	lib.SetPanicHandler(func(err interface{}) {
		glog.Panic("panic", zap.Any("err", err), zap.String("stack", string(debug.Stack())))
	})

	// 注册组件（注意顺序：glog 应该最先初始化，因为其他组件可能会使用日志）
	components := []component.IComponent[iface.INode]{
		logger.NewComponent(),
		actor.NewComponent(),
		cluster.NewComponent(),
	}

	//  注册组件
	components = append(components, comps...)
	for _, comp := range components {
		if err = n.IManager.Register(comp); err != nil {
			return err
		}
	}

	//  启动组件
	if err = n.IManager.Start(context.Background(), n); err != nil {
		return err
	}

	glog.Info("节点启动完成", zap.String("path", n.path), zap.Strings("component", n.IManager.GetComponentNames()))

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
	if err := n.IManager.Stop(ctx); err != nil {
		return err
	}
	shutdownTimeout := 30 * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()
	return lib.ShutdownGoroutines(timeoutCtx)
}
