package iface

import (
	"time"

	"github.com/dzm2020/gas/pkg/lib"
)

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
		NodeId() uint64
		NextID() uint64

		SessionFactory() ISessionFactory
		SetSessionFactory(f ISessionFactory)

		Serializer() lib.ISerializer
		Spawn(actor IActor, args ...interface{}) *Pid

		Register(ctx IContext) error
		Unregister(ctx IContext) error

		Named(ctx IContext) error
		Unname(ctx IContext) error

		SubmitTask(pid *Pid, task Task) (err error)
		SubmitTaskAndWait(pid *Pid, task Task, timeout time.Duration) (err error)
		Send(message *ActorMessage) (err error)
		Call(message *ActorMessage) (data []byte, err error)

		GetProcess(ref interface{}) IProcess
		GetAllProcesses() []IProcess
		ShutdownProcess(pid *Pid) error

		Shutdown() error
	}

	IContext interface {
		IMessageInvoker
		ID() *Pid
		Serializer() lib.ISerializer
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
		Forward(to *Pid, method string) error
		AfterFunc(duration time.Duration, task Task) *lib.Timer
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
		Raw() *Session
		SyncValues() error // 同步values
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
		FromRaw(ctx IContext, raw *Session) ISession
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

// Equal 自定义逻辑相等判断
func (o *Pid) Equal(other *Pid) bool {
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
