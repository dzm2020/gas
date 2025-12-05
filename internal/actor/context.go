// Package actor
// @Description:

package actor

import (
	"errors"
	"fmt"
	"gas/internal/iface"
	"gas/pkg/lib"
	"time"
)

type IMessageInvoker interface {
	InvokerMessage(message interface{}) error
}

// contextConfig context 配置
type contextConfig struct {
	pid         *iface.Pid
	actor       iface.IActor
	middlewares []iface.TaskMiddleware
	router      iface.IRouter
	system      iface.ISystem
	process     iface.IProcess
}

func newBaseActorContext(cfg *contextConfig) *baseActorContext {
	ctx := &baseActorContext{
		pid:          cfg.pid,
		actor:        cfg.actor,
		middlerWares: cfg.middlewares,
		router:       cfg.router,
		system:       cfg.system,
		process:      cfg.process,
		timerMgr:     newTimerManager(),
		node:         cfg.system.GetNode(),
	}
	return ctx
}

type baseActorContext struct {
	pid          *iface.Pid
	name         string
	actor        iface.IActor
	middlerWares []iface.TaskMiddleware
	msg          *iface.Message
	router       iface.IRouter
	system       iface.ISystem
	process      iface.IProcess // 保存自己的 process 引用
	timerMgr     *timerManager  // 定时器管理器
	node         iface.INode
}

func (a *baseActorContext) ID() *iface.Pid {
	return a.pid
}
func (a *baseActorContext) Node() iface.INode {
	return a.node
}
func (a *baseActorContext) GetRouter() iface.IRouter {
	return a.router
}
func (a *baseActorContext) System() iface.ISystem {
	return a.system
}
func (a *baseActorContext) SetRouter(router iface.IRouter) {
	a.router = router
}
func (a *baseActorContext) Message() *iface.Message {
	return a.msg
}
func (a *baseActorContext) InvokerMessage(msg interface{}) error {

	switch m := msg.(type) {
	case *iface.TaskMessage:
		return chain(a.middlerWares, m.Task)(a)
	case *iface.Message:
		a.msg = m
		if a.router != nil && a.router.HasRoute(int64(m.GetId())) {
			data, err := a.execHandler(m)
			m.Response(data, err)
			return err
		}
		return a.actor.OnMessage(a, m)
	case *iface.SyncMessage:
		a.msg = m.Message
		if a.router != nil && a.router.HasRoute(int64(m.GetId())) {
			data, err := a.execHandler(m.Message)
			m.Response(data, err)
			return err
		}
		return a.actor.OnMessage(a, m)
	default:
		return errors.New("not implement")
	}
}

func (a *baseActorContext) execHandler(msg *iface.Message) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("msg is nil")
	}
	session := msg.GetSession()
	return a.router.Handle(a, msg.GetId(), session, msg.GetData())
}

func (a *baseActorContext) Actor() iface.IActor {
	return a.actor
}

func (a *baseActorContext) GetSerializer() lib.ISerializer {
	if a.system != nil {
		return a.system.GetSerializer()
	}
	return lib.Json // 默认序列化器
}

func (a *baseActorContext) Exit() {
	// 取消所有定时器
	if a.timerMgr != nil {
		a.timerMgr.CancelAllTimers()
	}
	if a.system != nil {
		a.system.unregisterProcess(a.pid)
	}
	_ = a.actor.OnStop(a)
}

func (a *baseActorContext) Send(to *iface.Pid, msgId uint16, request interface{}) error {
	message := a.buildMessage(a.pid, to, msgId, request)
	return a.system.Send(message)
}

func (a *baseActorContext) Request(to *iface.Pid, msgId uint16, request interface{}, reply interface{}) error {
	message := a.buildMessage(a.pid, to, msgId, request)
	return a.system.Request(message, reply)
}

func (a *baseActorContext) Response(session *iface.Session, request interface{}) error {
	message := a.buildMessage(a.pid, session.GetAgent(), -1, request)
	if session.GetAgent() == a.pid {
		return a.InvokerMessage(message)
	}
	return a.system.Send(message)
}

func (a *baseActorContext) ResponseCode(session *iface.Session, code int64) error {
	session.Code = code
	message := a.buildMessage(a.pid, session.GetAgent(), -1, []byte{})
	if session.GetAgent() == a.pid {
		return a.InvokerMessage(message)
	}
	return a.system.Send(message)
}
func (a *baseActorContext) Forward(toPid *iface.Pid) error {
	return a.system.PushMessage(toPid, a.Message())
}

func (a *baseActorContext) buildMessage(from, to *iface.Pid, msgId uint16, request interface{}) *iface.Message {
	message := &iface.Message{
		To:   to,
		From: from,
		Id:   int64(msgId),
	}
	switch data := request.(type) {
	case []byte:
		message.Data = data
	default:
		ser := a.GetSerializer()
		requestData, err := ser.Marshal(request)
		if err != nil {
			return nil
		}
		message.Data = requestData
	}
	return message
}

func (a *baseActorContext) RegisterName(name string, isGlobal bool) error {
	return a.system.RegisterName(a.pid, a.process, name, isGlobal)
}

// RegisterTimer 注册定时器，时间到后通过 pushTask 通知 baseActorContext 然后执行回调
// 这是 IContext 接口要求的方法，内部调用 AfterFunc
func (a *baseActorContext) RegisterTimer(duration time.Duration, callback iface.Task) (int64, error) {
	return a.timerMgr.AfterFunc(a.process, duration, callback)
}

// AfterFunc 注册一次性定时器
func (a *baseActorContext) AfterFunc(duration time.Duration, callback iface.Task) (int64, error) {
	return a.timerMgr.AfterFunc(a.process, duration, callback)
}

// TickFunc 注册周期性定时器，每隔指定时间间隔执行一次回调
func (a *baseActorContext) TickFunc(interval time.Duration, callback iface.Task) (int64, error) {
	return a.timerMgr.TickFunc(a.process, interval, callback)
}

// CancelTimer 取消定时器
func (a *baseActorContext) CancelTimer(timerID int64) bool {
	return a.timerMgr.CancelTimer(timerID)
}

// CancelAllTimers 取消所有定时器
func (a *baseActorContext) CancelAllTimers() {
	a.timerMgr.CancelAllTimers()
}
