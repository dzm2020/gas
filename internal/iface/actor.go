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
		Named(name string) error
		Unname() error
		GetName() string
		Actor() IActor
		SetCallTimeout(timeout time.Duration)
		Send(to *Pid, methodName string, request interface{}) error
		Call(to *Pid, methodName string, request interface{}, reply interface{}) error
		Forward(to *Pid, method string) error
		AfterFunc(duration time.Duration, task Task) *lib.Timer
		Message() *ActorMessage
		Process() IProcess
		System() ISystem
		Shutdown() error
		Node() INode // 获取节点引用，用于序列化等操作
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
		Meta() *Session
		SetContext(ctx IContext)
		Response(request interface{}) error
		ResponseCode(code int64) error
		Push(cmd, act uint16, request interface{}) error
		Close() error
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
