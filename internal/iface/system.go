package iface

import (
	"gas/pkg/utils/serializer"
	"time"
)

type Task func(ctx IContext) error

type IContext interface {
	ID() *Pid
	Actor() IActor
	Send(to *Pid, msgId uint16, request interface{}) error
	Request(to *Pid, msgId uint16, request interface{}, reply interface{}) error
	Exit()
	GetSerializer() serializer.ISerializer
}

type IActor interface {
	OnInit(ctx IContext, params []interface{}) error
	OnMessage(ctx IContext, msg interface{}) error
	OnStop(ctx IContext) error
}

type IProcess interface {
	Context() IContext
	PushMessage(message interface{}) error
	PushTask(f Task) error
	PushTaskAndWait(timeout time.Duration, task Task) error
	Exit() error
}

type ISystem interface {
	SetNode(n INode)
	GetNode() INode
	GetSerializer() serializer.ISerializer
	SetSerializer(ser serializer.ISerializer)
	GetProcess(pid *Pid) IProcess
	GetProcessById(id uint64) IProcess
	GetProcessByName(name string) IProcess
	Send(message *Message) error
	Request(message *Message, timeout time.Duration) *RespondMessage
	GetAllProcesses() []IProcess
	//Spawn(producer Producer, options ...Option) (*Pid, IProcess)
}

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
