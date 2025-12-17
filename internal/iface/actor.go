package iface

import (
	"gas/pkg/lib"
	"time"
)

type (
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
		RegisterName(pid *Pid, process IProcess, name string) error
		HasName(name string) bool
		GetProcess(pid *Pid) IProcess
		GetProcessById(id uint64) IProcess
		GetProcessByName(name string) IProcess
		UnregisterName(pid *Pid)
		GetAllProcesses() []IProcess

		SubmitTask(to *Pid, task Task) (err error)
		SubmitTaskAndWait(to *Pid, task Task, timeout time.Duration) (err error)
		Send(message *ActorMessage) (err error)
		Call(message *ActorMessage) (data []byte, err error)

		Shutdown(timeout time.Duration) error
	}

	IContext interface {
		ID() *Pid
		InvokerMessage(msg interface{}) error
		Actor() IActor
		Send(to interface{}, methodName string, request interface{}) error
		Call(to interface{}, methodName string, request interface{}, reply interface{}) error
		AfterFunc(duration time.Duration, callback Task) *lib.Timer
		RegisterName(name string, isGlobal bool) error
		GetRouter() IRouter
		Message() *ActorMessage
		Process() IProcess
		Shutdown() error
	}
	IActor interface {
		OnInit(ctx IContext, params []interface{}) error
		OnMessage(ctx IContext, msg *Message) error
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
func (a *Actor) OnMessage(ctx IContext, msg *Message) error {
	return nil
}

func (p *Pid) IsLocal() bool {
	return p.NodeId == GetNode().GetID()
}
