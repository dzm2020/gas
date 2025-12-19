// Package actor 提供 Actor 模型实现，包括进程管理、消息路由、定时器等核心功能
package actor

import (
	"gas/internal/iface"
	"gas/internal/session"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/glog"
	"gas/pkg/lib"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"go.uber.org/zap"
)

// DefaultCallTimeout 默认调用超时时间
const DefaultCallTimeout = 3 * time.Second

var _ iface.IContext = (*actorContext)(nil)

type actorContext struct {
	process iface.IProcess // 保存自己的 process 引用
	pid     *iface.Pid
	actor   iface.IActor
	router  iface.IRouter
	msg     *iface.ActorMessage
	node    iface.INode
	system  iface.ISystem
	timeout time.Duration
}

func (a *actorContext) ID() *iface.Pid {
	return a.pid
}
func (a *actorContext) Node() iface.INode {
	return a.node
}

func (a *actorContext) Process() iface.IProcess {
	return a.process
}
func (a *actorContext) Actor() iface.IActor {
	return a.actor
}

func (a *actorContext) Message() *iface.ActorMessage {
	return a.msg
}
func (a *actorContext) InvokerMessage(msg interface{}) error {
	if err := a.dispatch(msg); err != nil {
		glog.Error("actor处理消息错误", zap.Any("pid", a.ID()), zap.Error(err))
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

// handleMessage 处理 Actor 消息
// 如果消息有对应的路由，则通过路由处理；否则调用 actor.OnMessage
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
	s := session.NewWithSession(msg.GetSession())
	s.SetContext(a)
	return a.router.Handle(a, msg.GetMethod(), s, msg.GetData())
}

func (a *actorContext) Send(pid *iface.Pid, methodName string, request interface{}) (err error) {
	toPid := a.system.Convert(pid, discovery.RouteRandom)

	var data []byte
	data, err = a.node.Marshal(request)
	if err != nil {
		return
	}

	message := iface.NewActorMessage(a.pid, toPid, methodName, data)
	message.Async = true
	return a.system.Send(message)
}

func (a *actorContext) SetCallTimeout(timeout time.Duration) {
	a.timeout = timeout
}

// Call 带超时的同步调用
func (a *actorContext) Call(to *iface.Pid, methodName string, request interface{}, reply interface{}) (err error) {
	var data []byte
	data, err = a.node.Marshal(request)
	if err != nil {
		return
	}

	message := iface.NewActorMessage(a.pid, to, methodName, data)
	message.Deadline = time.Now().Add(a.timeout).Unix()
	message.Async = false

	data, err = a.system.Call(message)
	if err != nil {
		return
	}
	return a.node.Unmarshal(data, reply)
}

func (a *actorContext) Forward(to *iface.Pid, method string) error {
	if a.Message() == nil {
		return ErrMessageIsNil
	}

	message := convertor.DeepClone(a.Message())
	message.To = to
	message.Method = method

	return a.system.Send(message)
}

func (a *actorContext) Named(name string) (err error) {
	return a.system.Named(name, a.pid)
}

func (a *actorContext) Unname() error {
	return a.system.Unname(a.pid)
}

// AfterFunc 注册一次性定时器
func (a *actorContext) AfterFunc(duration time.Duration, task iface.Task) *lib.Timer {
	return lib.AfterFunc(duration, func() {
		msg := iface.NewTaskMessage(task)
		if err := a.process.PostMessage(msg); err != nil {
			glog.Error("提交定时器任务失败", zap.Error(err))
		}
	})
}

func (a *actorContext) exit() {
	a.system.Remove(a.pid)
	// OnStop 的错误不影响进程退出流程
	if err := a.actor.OnStop(a); err != nil {
		glog.Error("Actor OnStop 回调失败", zap.Any("pid", a.pid), zap.Error(err))
	}
}

func (a *actorContext) Shutdown() error {
	return a.process.Shutdown()
}
