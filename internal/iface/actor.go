package iface

import (
	"gas/pkg/lib"
	"time"
)

type (
	IMessageInvoker interface {
		InvokerMessage(message interface{}) error
	}

	Task func(ctx IContext) error

	IProcess interface {
		Context() IContext
		Input(message interface{}) error
		Shutdown() error
	}

	ISystem interface {
		Spawn(actor IActor, args ...interface{}) *Pid
		Add(pid *Pid, process IProcess)
		Remove(pid *Pid)
		Named(name string, pid *Pid) error
		Unname(pid *Pid)
		HasName(name string) bool
		GetProcess(to interface{}) IProcess
		GetProcessById(id uint64) IProcess
		GetProcessByName(name string) IProcess
		GetAllProcesses() []IProcess
		SubmitTask(to interface{}, task Task) (err error)
		SubmitTaskAndWait(to interface{}, task Task, timeout time.Duration) (err error)
		Send(message *ActorMessage) (err error)
		Call(message *ActorMessage) (data []byte, err error)
		GenPid(to interface{}, strategy RouteStrategy) *Pid
		Shutdown(timeout time.Duration) error
	}

	IContext interface {
		IMessageInvoker
		ID() *Pid
		Named(name string) error
		Unname()
		Actor() IActor
		Send(to interface{}, methodName string, request interface{}) error
		Call(to interface{}, methodName string, request interface{}, reply interface{}) error
		AfterFunc(duration time.Duration, task Task) *lib.Timer
		Message() *ActorMessage
		Process() IProcess
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
