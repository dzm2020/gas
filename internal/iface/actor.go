package iface

import (
	"gas/pkg/lib"
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
	GetSerializer() lib.ISerializer
	// RegisterTimer 注册定时器，时间到后通过 pushTask 通知 baseActorContext 然后执行回调
	RegisterTimer(duration time.Duration, callback Task) (int64, error)
	// AfterFunc 注册一次性定时器
	AfterFunc(duration time.Duration, callback Task) (int64, error)
	// TickFunc 注册周期性定时器，每隔指定时间间隔执行一次回调
	TickFunc(interval time.Duration, callback Task) (int64, error)
	// CancelTimer 取消定时器
	CancelTimer(timerID int64) bool
	// CancelAllTimers 取消所有定时器
	CancelAllTimers()
	SendService(service string, msgId uint16, request interface{}, strategy RouteStrategy) error
	Node() INode
	RegisterName(name string, isGlobal bool) error
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
	GetSerializer() lib.ISerializer
	SetSerializer(ser lib.ISerializer)
	GetProcess(pid *Pid) IProcess
	GetProcessById(id uint64) IProcess
	GetProcessByName(name string) IProcess
	Send(message *Message) error
	Request(message *Message, timeout time.Duration) *RespondMessage
	GetAllProcesses() []IProcess
	Spawn(actor IActor, args ...interface{}) *Pid
	PushTask(pid *Pid, f Task) error
	PushTaskAndWait(pid *Pid, timeout time.Duration, task Task) error
	PushMessage(pid *Pid, message interface{}) error
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
