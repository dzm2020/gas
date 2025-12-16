package node

import (
	"context"
	"gas/internal/actor"
	"gas/internal/iface"
	"gas/internal/remote"
	discoveryFactory "gas/pkg/discovery"
	"gas/pkg/lib/glog"
	messageQueFactory "gas/pkg/messageQue"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogComponent glog 日志组件
type LogComponent struct {
}

// NewLogComponent 创建 glog 组件
func NewLogComponent() iface.IComponent {
	return &LogComponent{}
}

func (c *LogComponent) Name() string {
	return "log"
}
func (c *LogComponent) Start(ctx context.Context) error {

	cfg := iface.GetNode().GetConfig()

	// 使用节点配置中的 glog 配置初始化
	if err := glog.InitFromConfig(cfg.Logger); err != nil {
		return err
	}
	options := []zap.Option{
		zap.Fields(zap.String("nodeKind", iface.GetNode().GetKind()), zap.Uint64("nodeId", iface.GetNode().GetID())),
		zap.Hooks(func(entry zapcore.Entry) error {
			if entry.Level >= zap.DPanicLevel {
				iface.GetNode().CallPanicHook(entry)
			}
			return nil
		}),
	}
	glog.WithOptions(options...)
	return nil
}
func (c *LogComponent) Stop(ctx context.Context) error {
	glog.Stop()
	return nil
}

// RemoteComponent 远程通信组件
type RemoteComponent struct {
	iface.IRemote
}

// NewRemoteComponent 创建 remote 组件
func NewRemoteComponent() *RemoteComponent {
	c := &RemoteComponent{}
	return c
}

func (r *RemoteComponent) Name() string {
	return "remote"
}

func (r *RemoteComponent) Start(ctx context.Context) error {
	config := iface.GetNode().GetConfig()
	// 创建服务发现实例
	discoveryInstance, err := discoveryFactory.NewFromConfig(*config.Remote.Discovery)
	if err != nil {
		return err
	}

	// 创建远程通信管理器
	messageQueue, err := messageQueFactory.NewFromConfig(*config.Remote.MessageQueue)
	if err != nil {
		return err
	}

	r.IRemote = remote.New(discoveryInstance, messageQueue, config.Remote.SubjectPrefix)
	//  建立引用
	iface.GetNode().SetRemote(r.IRemote)
	//  注册节点并订阅
	if err = r.IRemote.Start(ctx); err != nil {
		return err
	}
	return nil
}

func (r *RemoteComponent) Stop(ctx context.Context) error {
	iface.GetNode().SetRemote(nil)
	return r.IRemote.Shutdown(ctx)
}

// ActorComponent actor 系统组件适配器
type ActorComponent struct {
	iface.ISystem
}

// NewActorComponent 创建 actor 组件
func NewActorComponent() *ActorComponent {
	return &ActorComponent{}
}

func (a *ActorComponent) Name() string {
	return "actorSystem"
}

func (a *ActorComponent) Start(ctx context.Context) error {
	a.ISystem = actor.NewSystem()
	iface.GetNode().SetSystem(a.ISystem)
	return nil
}

func (a *ActorComponent) Stop(ctx context.Context) error {
	iface.GetNode().SetSystem(nil)
	return a.ISystem.Shutdown(10 * time.Second)
}
