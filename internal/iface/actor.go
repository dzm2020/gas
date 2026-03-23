package iface

import (
	"time"

	"github.com/dzm2020/gas/api/pb"
	"github.com/dzm2020/gas/pkg/lib/serializer"
	"github.com/dzm2020/gas/pkg/lib/timer"
)

type Pid = pb.Pid

type (
	IMessageInvoker interface {
		InvokerMessage(message interface{}) error
	}

	Task func(ctx IContext) error

	IProcess interface {
		PostMessage(message IMessage) error
		Shutdown() error
	}

	ISystem interface {
		IEventBus

		NodeId() uint64
		NextID() uint64

		SessionFactory() ISessionFactory
		SetSessionFactory(f ISessionFactory)

		Serializer() serializer.ISerializer
		Spawn(actor IActor, args ...interface{}) *Pid

		Register(ctx IContext) error
		Unregister(ctx IContext) error

		Named(ctx IContext) error
		Unname(ctx IContext) error

		SubmitTask(pid *Pid, task Task) (err error)
		SubmitTaskAndWait(pid *Pid, task Task, timeout time.Duration) (err error)

		SendMessage(message *ActorMessage) (err error)
		CallMessage(message *ActorMessage) (data []byte, err error)
		Send(from, to *Pid, methodName string, request interface{}) (err error)
		Call(from, to *Pid, methodName string, request interface{}, reply interface{}, timeout time.Duration) (err error)

		GetProcess(ref interface{}) IProcess
		GetAllProcesses() []IProcess
		ShutdownProcess(pid *Pid) error

		Shutdown() error
	}

	IContext interface {
		IMessageInvoker
		ID() *Pid
		Serializer() serializer.ISerializer
		Named(name string) error
		Unname() error
		GetName() string
		Actor() IActor
		Message() *ActorMessage
		Process() IProcess
		System() ISystem
		SetCallTimeout(timeout time.Duration)
		Send(to *Pid, methodName string, request interface{}) error
		Call(to *Pid, methodName string, request interface{}, reply interface{}) error
		SendMessage(message *ActorMessage) (err error)
		CallMessage(message *ActorMessage) (data []byte, err error)
		// Subscribe 在本 Actor（ctx.ID()）上订阅事件；PublishLocal / PublishCluster 委托本节点 ISystem。
		Subscribe(topic string, handler EventHandler) (IEventSubscription, error)
		PublishLocal(topic string, payload []byte)
		PublishCluster(topic string, payload []byte) error
		ForwardMessage(pid *Pid, methodName string) error
		AfterFunc(duration time.Duration, task Task) *timer.Timer
		Shutdown() error
	}
	IActor interface {
		OnInit(ctx IContext, params []interface{}) error
		OnMessage(ctx IContext, msg interface{}) error
		OnStop(ctx IContext) error
	}

	IRouter interface {
		Handle(ctx IContext, methodName string, session ISession, data []byte) ([]byte, error)
		HasRoute(methodName string) bool
		AutoRegister(actor IActor)
	}

	ISession interface {
		GetId() int64
		Raw() *pb.Session
		SetString(key, value string)
		GetString(key string) string
		SetUint64(key string, value uint64)
		GetUint64(key string) uint64
		SetInt64(key string, value int64)
		GetInt64(key string) int64
	}

	// ISessionFactory 由上层（如 gate）实现，用于在 actor 处理消息时把 *Session 包装成可写的 ISession。
	// actor 包仅依赖此接口，不依赖具体 session 实现。
	ISessionFactory interface {
		FromRaw(ctx IContext, raw *pb.Session) ISession
	}
)

var _ IActor = (*Actor)(nil)

type Actor struct {
}

func (a *Actor) OnInit(ctx IContext, params []interface{}) error {
	return nil
}
func (a *Actor) OnStop(ctx IContext) error {
	return nil
}
func (a *Actor) OnMessage(ctx IContext, msg interface{}) error {
	return nil
}

// EqualPid 自定义逻辑相等判断
func EqualPid(o *Pid, other *Pid) bool {
	// 先比对可比较字段
	if o == nil && other == nil {
		return true
	}
	if o == nil || other == nil {
		return false
	}
	if o.GetNodeId() != other.GetNodeId() {
		return false
	}
	if o.GetActorId() == other.GetActorId() {
		return true
	}
	return o.GetActorName() == other.GetActorName()
}
