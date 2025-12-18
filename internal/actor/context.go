// Package actor 提供 Actor 模型实现，包括进程管理、消息路由、定时器等核心功能
package actor

import (
	"gas/internal/errs"
	"gas/internal/iface"
	"gas/internal/session"
	"gas/pkg/lib"
	"gas/pkg/lib/glog"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"go.uber.org/zap"
)

type actorContext struct {
	process iface.IProcess // 保存自己的 process 引用
	pid     *iface.Pid
	actor   iface.IActor
	router  iface.IRouter
	msg     *iface.ActorMessage
}

func (a *actorContext) ID() *iface.Pid {
	return a.pid
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
	if err := a.dispatch(msg); err != nil {
		glog.Error("InvokerMessage", zap.Any("pid", a.ID()), zap.Error(err))
		return err
	}
	return nil
}

func (a *actorContext) dispatch(msg interface{}) error {
	switch m := msg.(type) {
	case *iface.TaskMessage:
		return m.Task(a)
	case *iface.ActorMessage:
		return a.handleMessage(m)
	}
	return a.actor.OnMessage(a, msg)
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
	a.msg = nil
	return err
}

// execHandler 基于方法名执行处理器
func (a *actorContext) execHandler(msg *iface.Message) ([]byte, error) {
	s := session.New(msg.GetSession())
	s.SetContext(a)
	return a.router.Handle(a, msg.GetMethod(), s, msg.GetData())
}

func (a *actorContext) Send(to interface{}, methodName string, request interface{}) error {
	node := iface.GetNode()
	system := node.System()

	toPid := system.GenPid(to, iface.RouteRandom)

	message := iface.NewActorMessage(a.pid, toPid, methodName, node.Marshal(request))
	message.Async = true
	return system.Send(message)
}

func (a *actorContext) Call(to interface{}, methodName string, request interface{}, reply interface{}) error {
	node := iface.GetNode()
	system := node.System()

	toPid := system.GenPid(to, iface.RouteRandom)

	message := iface.NewActorMessage(a.pid, toPid, methodName, node.Marshal(request))
	message.Deadline = time.Now().Add(time.Second * 3).Unix()
	message.Async = false

	data, err := system.Call(message)
	node.Unmarshal(data, reply)
	return err
}

func (a *actorContext) Forward(to interface{}, method string) error {
	node := iface.GetNode()
	system := node.System()
	if a.Message() == nil {
		return errs.ErrMessageIsNil
	}
	toPid := system.GenPid(to, iface.RouteRandom)
	message := convertor.DeepClone(a.Message())
	message.To = toPid
	message.Method = method

	return system.Send(message)
}

func (a *actorContext) Named(name string) (err error) {
	node := iface.GetNode()
	system := node.System()
	return system.Named(name, a.pid)
}

func (a *actorContext) Unname() {
	node := iface.GetNode()
	system := node.System()
	system.Unname(a.pid)
}

// AfterFunc 注册一次性定时器
func (a *actorContext) AfterFunc(duration time.Duration, task iface.Task) *lib.Timer {
	return lib.AfterFunc(duration, func() {
		msg := iface.NewTaskMessage(task)
		_ = a.process.Input(msg)
	})
}

func (a *actorContext) exit() {
	node := iface.GetNode()
	system := node.System()

	system.Remove(a.pid)
	_ = a.actor.OnStop(a)
}

func (a *actorContext) Shutdown() error {
	return a.process.Shutdown()
}
