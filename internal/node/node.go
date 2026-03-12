package node

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	compcluster "github.com/dzm2020/gas/internal/component/cluster"
	complogger "github.com/dzm2020/gas/internal/component/logger"
	compprofile "github.com/dzm2020/gas/internal/component/profile"
	compsystem "github.com/dzm2020/gas/internal/component/system"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/component"
	"github.com/dzm2020/gas/pkg/lib/grs"
	"github.com/dzm2020/gas/pkg/lib/serializer"
	"github.com/dzm2020/gas/pkg/lib/xerror"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 创建节点实例
func New(path string) *Node {
	node := &Node{
		Member:     new(iface.Member),
		serializer: serializer.Json,
		IManager:   component.NewComponentsMgr[iface.INode](),
		path:       path,
	}
	return node
}

var _ iface.INode = (*Node)(nil)

type Node struct {
	*iface.Member
	component.IManager[iface.INode]
	path       string
	serializer serializer.ISerializer
	panicHook  func(entry zapcore.Entry)
}

func (n *Node) Info() *iface.Member {
	return n.Member
}

func (n *Node) System() iface.ISystem {
	c := n.GetComponent(compsystem.Name)
	if c == nil {
		return nil
	}
	return c.(iface.ISystem)
}

func (n *Node) Cluster() iface.ICluster {
	c := n.GetComponent(compcluster.Name)
	if c == nil {
		return nil
	}
	return c.(iface.ICluster)
}

func (n *Node) Profile() iface.IProfile {
	c := n.GetComponent(compprofile.Name)
	if c == nil {
		return nil
	}
	return c.(iface.IProfile)
}

// SetSerializer 设置序列化器
func (n *Node) SetSerializer(ser serializer.ISerializer) {
	n.serializer = ser
}
func (n *Node) SetPanicHook(hook func(entry zapcore.Entry)) {
	n.panicHook = hook
}
func (n *Node) Serializer() serializer.ISerializer {
	return n.serializer
}

func (n *Node) Startup(comps ...component.IComponent[iface.INode]) (err error) {
	defer xerror.PrintCoreDump()

	grs.SetPanicHandler(func(err interface{}) {
		glog.Panic("panic", zap.Any("err", err), zap.String("stack", string(debug.Stack())))
	})

	// 注册组件（Profile 需为首个，负责加载配置并填充 node.Member）
	components := []component.IComponent[iface.INode]{
		compprofile.New(n.path),
		complogger.New(n.panicHook),
		compcluster.New(),
		compsystem.New(),
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

	//  所有组件注册完成后,再在集群中注册节点
	if err = n.Cluster().Register(n.Info()); err != nil {
		return xerror.Wrapf(err, "discovery Register fail")
	}

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
