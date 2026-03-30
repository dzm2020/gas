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
	"github.com/dzm2020/gas/pkg/lib/uid"
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
	node.ctx, node.cancel = context.WithCancel(context.Background())
	node.init()
	return node
}

var _ iface.INode = (*Node)(nil)

type Node struct {
	*iface.Member
	component.IManager[iface.INode]
	path       string
	serializer serializer.ISerializer
	panicHook  func(entry zapcore.Entry)
	ctx        context.Context
	cancel     context.CancelFunc
	components []component.IComponent[iface.INode]
}

func (n *Node) init() {
	//  初始配置文件
	profile := compprofile.New(n.path)
	//  初始节点信息
	if err := profile.Get("node", n.Member); err != nil {
		panic(err)
	}

	//  初始化日志
	logger := complogger.New(func(entry zapcore.Entry) {
		if n.panicHook != nil {
			n.panicHook(entry)
		}
	})

	// 注册组件（Profile 需为首个，负责加载配置并填充 node.Member）
	n.components = []component.IComponent[iface.INode]{
		profile,
		logger,
	}
	//  初始化集群
	if !profile.Standalone() {
		n.components = append(n.components, compcluster.New())
	}
	//  初始化actor system
	n.components = append(n.components, compsystem.New())

	uid.Init(int64(n.GetID()))
	grs.SetPanicHandler(func(err interface{}) {
		glog.Panic("panic", zap.Any("err", err), zap.String("stack", string(debug.Stack())))
	})

	glog.Info("节点初始化完成", zap.String("path", n.path))
}

func (n *Node) Info() *iface.Member {
	return n.Member
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

func (n *Node) Startup(comps ...component.IComponent[iface.INode]) (err error) {
	defer xerror.PrintCoreDump()

	components := n.components
	components = append(components, comps...)
	for _, comp := range components {
		if err = n.IManager.Register(comp); err != nil {
			return
		}
	}

	//  启动组件
	if err = n.IManager.Start(n.ctx, n); err != nil {
		glog.Error("组件启动失败", zap.Error(err))
		return
	}

	glog.Info("节点启动完成", zap.Strings("component", n.IManager.GetComponentNames()))

	if !n.Profile().Standalone() {
		if err = n.Cluster().Register(n.Info()); err != nil {
			return err
		}
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
	if err := n.IManager.Stop(n.ctx); err != nil {
		glog.Error("组件停止失败", zap.Error(err))
		return err
	}
	timeout := 30 * time.Second
	timeoutCtx, cancel := context.WithTimeout(n.ctx, timeout)
	defer cancel()
	defer n.cancel()
	return grs.Shutdown(timeoutCtx)
}
