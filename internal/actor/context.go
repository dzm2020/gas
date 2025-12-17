// Package actor 提供 Actor 模型实现，包括进程管理、消息路由、定时器等核心功能
package actor

import (
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
	ctx := &actorContext{
		router: nil, // 稍后在 Spawn 时根据 actor 类型设置
	}

	return ctx
}

type actorContext struct {
	isGlobalName bool
	name         string
	process      iface.IProcess // 保存自己的 process 引用
	pid          *iface.Pid
	actor        iface.IActor
	router       iface.IRouter
	msg          *iface.ActorMessage
}

func (a *actorContext) ID() *iface.Pid {
	return a.pid
}

func (a *actorContext) GetRouter() iface.IRouter {
	return a.router
}

func (a *actorContext) Process() iface.IProcess {
	return a.process
}
func (a *actorContext) Actor() iface.IActor {
	return a.actor
}

func (a *actorContext) Message() *iface.ActorMessage {
	if a.msg == nil {
		return nil
	}
	return a.msg
}
func (a *actorContext) InvokerMessage(msg interface{}) error {
	if err := a.invokerMessage(msg); err != nil {
		glog.Error("InvokerMessage", zap.Any("pid", a.ID()), zap.Error(err))
		return err
	}
	return nil
}

func (a *actorContext) invokerMessage(msg interface{}) error {
	switch m := msg.(type) {
	case *iface.TaskMessage:
		return m.Task(a)
	case *iface.ActorMessage:
		glog.Debug("InvokerMessage", zap.Any("pid", a.ID()), zap.Any("msg", msg))
		return a.handleMessage(m)
	default:
		return errs.ErrUnsupportedMessageType(fmt.Sprintf("%T", msg))
	}
}

// handleMessage
func (a *actorContext) handleMessage(m *iface.ActorMessage) error {
	a.msg = m
	methodName := m.Message.GetMethod()
	if a.router != nil && methodName != "" && a.router.HasRoute(methodName) {
		data, err := a.execHandler(m.Message)
		m.Response(data, err)
		return err
	}
	// 如果没有路由，调用 actor.OnMessage
	err := a.actor.OnMessage(a, m.Message)
	m.Response(nil, err)
	return err
}

// execHandler 基于方法名执行处理器
func (a *actorContext) execHandler(msg *iface.Message) ([]byte, error) {
	session := iface.NewSession(a, msg.GetSession())
	return a.router.Handle(a, msg.GetMethod(), session, msg.GetData())
}

func (a *actorContext) Send(to interface{}, methodName string, request interface{}) error {
	node := iface.GetNode()
	toPid := node.CastPid(to)
	message := iface.NewActorMessage(a.pid, toPid, methodName, node.Marshal(request))
	message.Async = true
	return node.Send(message)
}

func (a *actorContext) Call(to interface{}, methodName string, request interface{}, reply interface{}) error {
	node := iface.GetNode()
	toPid := node.CastPid(to)

	message := iface.NewActorMessage(a.pid, toPid, methodName, node.Marshal(request))
	message.Deadline = time.Now().Add(time.Second * 3).Unix()
	message.Async = false

	data, err := node.Call(message)
	iface.GetNode().Unmarshal(data, reply)
	return err
}

func (a *actorContext) RegisterName(name string, global bool) error {
	system := iface.GetNode().System()

	//  本地注册
	if err := system.RegisterName(a.pid, a.process, name); err != nil {
		return err
	}
	//  集群注册
	if global {
		iface.GetNode().AddTag(name)
		if err := iface.GetNode().Update(); err != nil {
			return err
		}
	}
	a.name = name
	a.isGlobalName = global

	return nil
}

// AfterFunc 注册一次性定时器
func (a *actorContext) AfterFunc(duration time.Duration, task iface.Task) *lib.Timer {
	return lib.AfterFunc(duration, func() {
		msg := iface.NewTaskMessage(task)
		_ = a.process.Input(msg)
	})
}

func (a *actorContext) exit() {
	system := iface.GetNode().System()
	//  本地移除
	system.Remove(a.pid)
	//  移除全局名字
	if a.isGlobalName {
		iface.GetNode().RemoteTag(a.name)
		_ = iface.GetNode().Update()
	}
	_ = a.actor.OnStop(a)
}

func (a *actorContext) Shutdown() error {
	return a.process.Shutdown()
}
