package node

import (
	"context"
	"github.com/dzm2020/gas/internal/actor"
	"github.com/dzm2020/gas/internal/cluster"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/internal/logger"
	"github.com/dzm2020/gas/internal/profile"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib"
	"github.com/dzm2020/gas/pkg/lib/component"
	"github.com/dzm2020/gas/pkg/lib/grs"
	"github.com/dzm2020/gas/pkg/lib/xerror"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	_ "github.com/dzm2020/gas/pkg/discovery/provider/consul"
	_ "github.com/dzm2020/gas/pkg/messageQue/provider/nats"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 创建节点实例
func New(path string) *Node {
	node := &Node{
		Member:     new(iface.Member),
		serializer: lib.Json,
		IManager:   component.NewComponentsMgr[iface.INode](),
		path:       path,
		viper:      viper.New(),
	}
	return node
}

type Node struct {
	*iface.Member
	component.IManager[iface.INode]
	path       string
	system     iface.ISystem
	cluster    iface.ICluster
	serializer lib.ISerializer
	panicHook  func(entry zapcore.Entry)
	viper      *viper.Viper
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
func (n *Node) SetPanicHook(hook func(entry zapcore.Entry)) {
	n.panicHook = hook
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

func (n *Node) Startup(comps ...component.IComponent[iface.INode]) (err error) {
	defer xerror.PrintCoreDump()

	grs.SetPanicHandler(func(err interface{}) {
		glog.Panic("panic", zap.Any("err", err), zap.String("stack", string(debug.Stack())))
	})

	profile.Init(n.path)

	if err = profile.Get("node", n.Member); err != nil {
		return
	}

	// 注册组件
	components := []component.IComponent[iface.INode]{
		logger.NewComponent(n.panicHook),
		actor.NewComponent(),
		cluster.NewComponent(),
	}

	components = append(components, comps...)
	for _, comp := range components {
		if err = n.IManager.Register(comp); err != nil {
			return
		}
	}

	//  启动组件
	if err = n.IManager.Start(context.Background(), n); err != nil {
		glog.Error("组件启动失败", zap.Error(err))
		return
	}

	glog.Info("节点启动完成", zap.String("path", n.path), zap.Strings("component", n.IManager.GetComponentNames()))

	// 阻塞等待进程终止信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL, syscall.SIGTERM)
	<-sigChan

	return n.shutdown()
}

// Shutdown 优雅关闭节点，关闭所有组件
func (n *Node) shutdown() error {
	defer glog.Info("节点停止运行完成")
	glog.Info("节点开始停止运行")
	if err := n.IManager.Stop(context.Background()); err != nil {
		glog.Error("组件停止失败", zap.Error(err))
		return err
	}
	shutdownTimeout := 30 * time.Second
	timeoutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return grs.Shutdown(timeoutCtx)
}
