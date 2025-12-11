// Package actor 提供 Actor 模型实现，包括进程管理、消息路由、定时器等核心功能
package actor

import (
	"errors"
	"fmt"
	"gas/internal/errs"
	"gas/internal/iface"
	"gas/pkg/lib"
	"gas/pkg/lib/glog"
	"time"

	"go.uber.org/zap"
)

type IMessageInvoker interface {
	InvokerMessage(message interface{}) error
}

func newActorContext() *actorContext {
	ctx := &actorContext{}
	return ctx
}

type actorContext struct {
	system  *System
	process iface.IProcess // 保存自己的 process 引用
	pid     *iface.Pid
	name    string
	actor   iface.IActor
	router  iface.IRouter
	msg     *iface.Message
}

func (a *actorContext) ID() *iface.Pid {
	return a.pid
}

func (a *actorContext) GetRouter() iface.IRouter {
	return a.router
}
func (a *actorContext) System() iface.ISystem {
	return a.system
}
func (a *actorContext) SetRouter(router iface.IRouter) {
	a.router = router
}
func (a *actorContext) Process() iface.IProcess {
	return a.process
}
func (a *actorContext) Actor() iface.IActor {
	return a.actor
}

func (a *actorContext) GetSerializer() lib.ISerializer {
	if a.system != nil {
		return a.system.GetSerializer()
	}
	return lib.Json // 默认序列化器
}

func (a *actorContext) Message() *iface.Message {
	if a.msg == nil {
		return nil
	}
	return a.msg
}
func (a *actorContext) InvokerMessage(msg interface{}) error {
	if err := a.invokerMessage(msg); err != nil {
		glog.Error("InvokerMessage", zap.Any("msg", msg), zap.Error(err))
		return err
	}
	return nil
}

func (a *actorContext) invokerMessage(msg interface{}) error {
	switch m := msg.(type) {
	case *iface.TaskMessage:
		return m.Task(a)
	case *iface.Message:
		return a.handleMessage(m)
	case *iface.SyncMessage:
		return a.handleSyncMessage(m)
	default:
		return errs.ErrUnsupportedMessageType(fmt.Sprintf("%T", msg))
	}
}

// handleMessage 处理普通消息
func (a *actorContext) handleMessage(m *iface.Message) error {
	a.msg = m
	if a.router != nil && a.router.HasRoute(m.GetId()) {
		_, err := a.execHandler(m)
		return err
	}
	return a.actor.OnMessage(a, m)
}

// handleSyncMessage 处理同步消息
func (a *actorContext) handleSyncMessage(m *iface.SyncMessage) error {
	a.msg = m.Message
	if a.router != nil && a.router.HasRoute(m.GetId()) {
		data, err := a.execHandler(m.Message)
		m.Response(data, err)
		return err
	}
	// 如果没有路由，调用 actor.OnMessage
	err := a.actor.OnMessage(a, m.Message)
	m.Response(nil, err)
	return err
}

func (a *actorContext) execHandler(msg *iface.Message) ([]byte, error) {
	if msg == nil {
		return nil, errs.ErrMsgIsNil()
	}
	session := msg.GetSession()
	wrapSession := iface.NewSession(a, session)
	return a.router.Handle(a, msg.GetId(), wrapSession, msg.GetData())
}

func (a *actorContext) Send(to *iface.Pid, msgId uint16, request interface{}) error {
	message := iface.NewMessage(a.pid, to, int64(msgId))

	data, err := iface.Marshal(a.GetSerializer(), request)
	if err != nil {
		return err
	}
	message.Data = data

	return a.system.Send(message)
}

func (a *actorContext) Call(to *iface.Pid, msgId uint16, request interface{}, reply interface{}) error {
	message := iface.NewMessage(a.pid, to, int64(msgId))

	data, err := iface.Marshal(a.GetSerializer(), request)
	if err != nil {
		return err
	}
	message.Data = data

	response := a.system.Call(message, time.Second*3)
	if response.Error != "" {
		return errors.New(response.Error)
	}
	return iface.Unmarshal(a.GetSerializer(), response.GetData(), reply)
}

func (a *actorContext) RegisterName(name string, isGlobal bool) error {
	return a.system.RegisterName(a.pid, a.process, name, isGlobal)
}

// AfterFunc 注册一次性定时器
func (a *actorContext) AfterFunc(duration time.Duration, callback iface.Task) *lib.Timer {
	return lib.AfterFunc(duration, func() {
		_ = a.process.PushTask(callback)
	})
}

func (a *actorContext) exit() {
	a.system.Remove(a.pid)
	_ = a.actor.OnStop(a)
}

func (a *actorContext) Shutdown() error {
	return a.process.Shutdown()
}
