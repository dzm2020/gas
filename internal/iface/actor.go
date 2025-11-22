package iface

import (
	"gas/pkg/utils/serializer"
	"time"
)

type Task func(ctx IContext) error
type TaskMiddleware func(Task) Task

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
	Spawn(actor IActor, options ...Option) (*Pid, IProcess)
}

type IRouter interface {
	Register(msgId uint16, handler interface{}) error
	Handle(ctx IContext, msgId uint16, data []byte) ([]byte, error) // 返回 response 数据和错误，异步调用时 response 为 nil
	HasRoute(msgId uint16) bool                                     // 判断指定消息ID的路由是否存在
}

// NewErrorResponse 创建错误响应消息
func NewErrorResponse(errMsg string) *RespondMessage {
	return &RespondMessage{
		Error: errMsg,
	}
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
