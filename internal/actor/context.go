// Package actor 提供 Actor 模型实现，包括进程管理、消息路由、定时器等核心功能
package actor

import (
	"errors"
	"time"

	"github.com/dzm2020/gas/api/pb"
	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/serializer"
	"github.com/dzm2020/gas/pkg/lib/timer"

	"go.uber.org/zap"
)

// DefaultCallTimeout 默认调用超时时间
const DefaultCallTimeout = 3 * time.Second

var _ iface.IContext = (*actorContext)(nil)

type actorContext struct {
	process    iface.IProcess // 保存自己的 process 引用
	pid        *iface.Pid
	actor      iface.IActor
	router     iface.IRouter
	msg        *iface.ActorMessage
	system     iface.ISystem
	timeout    time.Duration
	serializer serializer.ISerializer
}

func (a *actorContext) ID() *iface.Pid {
	return a.pid
}

func (a *actorContext) System() iface.ISystem {
	return a.system
}

func (a *actorContext) Subscribe(topic string, handler iface.EventHandler) (iface.IEventSubscription, error) {
	if a.system == nil {
		return nil, errors.New("event: system 未初始化")
	}
	return a.system.Subscribe(topic, a.ID(), handler)
}

func (a *actorContext) PublishLocal(topic string, payload []byte) {
	if a.system == nil {
		return
	}
	a.system.PublishLocal(topic, payload)
}

func (a *actorContext) PublishCluster(topic string, payload []byte) error {
	if a.system == nil {
		return ErrEventNoCluster
	}
	return a.system.PublishCluster(topic, payload)
}

func (a *actorContext) Process() iface.IProcess {
	return a.process
}
func (a *actorContext) Actor() iface.IActor {
	return a.actor
}

func (a *actorContext) Serializer() serializer.ISerializer {
	return a.serializer
}

func (a *actorContext) GetName() string {
	return a.pid.GetActorName()
}

func (a *actorContext) Named(name string) (err error) {
	a.pid.ActorName = name
	return a.system.Named(a)
}

func (a *actorContext) Unname() error {
	if err := a.system.Unname(a); err != nil {
		return err
	}
	a.pid.ActorName = ""
	return nil
}

// AfterFunc 注册一次性定时器
func (a *actorContext) AfterFunc(duration time.Duration, task iface.Task) *timer.Timer {
	return timer.AfterFunc(duration, func() {
		msg := iface.NewTaskMessage(task)
		if err := a.process.PostMessage(msg); err != nil {
			glog.Error("提交定时器任务失败", zap.Error(err))
		}
	})
}

func (a *actorContext) Message() *iface.ActorMessage {
	return a.msg
}

func (a *actorContext) InvokerMessage(msg interface{}) error {
	//  重置下消息
	a.msg = &iface.ActorMessage{Message: &pb.Message{}}
	//  处理消息
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
	glog.Debug("actor没有找到消息路由,执行默认方法", zap.Any("pid", a.ID()), zap.String("method", methodName))
	return err
}

// execHandler 基于方法名执行处理器
func (a *actorContext) execHandler(msg *pb.Message) ([]byte, error) {
	s := NewSession(a, msg.GetSession())
	return a.router.Handle(a, msg.GetMethod(), s, msg.GetData())
}

func (a *actorContext) Send(pid *iface.Pid, methodName string, request interface{}) (err error) {
	return a.system.Send(a.pid, pid, methodName, request)
}

func (a *actorContext) SetCallTimeout(timeout time.Duration) {
	a.timeout = timeout
}

// Call 带超时的同步调用（超时由 SetCallTimeout 设置，未设置时使用系统默认）
func (a *actorContext) Call(to *iface.Pid, methodName string, request interface{}, reply interface{}) (err error) {
	return a.system.Call(a.pid, to, methodName, request, reply, a.timeout)
}

func (a *actorContext) SendMessage(message *iface.ActorMessage) (err error) {
	return a.system.SendMessage(message)
}
func (a *actorContext) CallMessage(message *iface.ActorMessage) (data []byte, err error) {
	return a.system.CallMessage(message)
}

func (a *actorContext) ForwardMessage(pid *iface.Pid, methodName string) error {
	msg := a.Message()
	msg.To = pid
	msg.Method = methodName
	return a.system.SendMessage(msg)
}

func (a *actorContext) Shutdown() error {
	return a.process.Shutdown()
}
