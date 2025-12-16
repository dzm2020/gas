// Package actor 提供 Actor 模型实现，包括进程管理、消息路由、定时器等核心功能
package actor

import (
	"gas/internal/errs"
	"gas/internal/iface"
	"sync/atomic"
	"time"
)

// System Actor 系统，管理所有进程和消息传递
type System struct {
	*Manager     // 名字管理器
	uniqId       atomic.Uint64
	shuttingDown atomic.Bool
}

// NewSystem 创建新的 Actor 系统
func NewSystem() *System {
	sys := &System{
		uniqId:  atomic.Uint64{},
		Manager: NewNameManager(),
	}
	return sys
}

// checkShuttingDown 检查系统是否正在关闭
func (s *System) checkShuttingDown() error {
	if s.shuttingDown.Load() {
		return errs.ErrSystemShuttingDown
	}
	return nil
}

// newPid 创建新的进程ID
func (s *System) newPid() *iface.Pid {
	return &iface.Pid{
		ServiceId: s.uniqId.Add(1),
		NodeId:    iface.GetNode().GetID(),
	}
}

// Spawn 创建新的 actor 进程
func (s *System) Spawn(actor iface.IActor, args ...interface{}) *iface.Pid {
	ctx := newActorContext()
	pid := s.newPid()
	mailBox := NewMailbox()
	process := NewProcess(ctx, mailBox)
	ctx.process = process
	ctx.pid = pid
	ctx.actor = actor
	// 使用全局 router 管理器，同一个类型的 actor 共享同一个 router
	ctx.router = GetRouterForActor(actor)
	mailBox.RegisterHandlers(ctx, NewDefaultDispatcher(1024))
	s.Add(pid, process)
	_ = process.PushTask(func(ctx iface.IContext) error {
		return ctx.Actor().OnInit(ctx, args)
	})
	return pid
}

// isLocalPid 判断进程 ID 是否为本地进程
func (s *System) isLocalPid(pid *iface.Pid) bool {
	return pid.GetNodeId() == iface.GetNode().GetID()
}

// Send 发送消息到指定进程
func (s *System) Send(message *iface.ActorMessage) error {
	if err := s.checkShuttingDown(); err != nil {
		return err
	}

	to := message.GetTo()
	if s.isLocalPid(to) {
		// 本地消息，直接发送到本地进程
		process := s.GetProcess(to)
		if process == nil {
			return errs.ErrProcessNotFound
		}
		return process.Send(message)
	} else {
		// 远程消息，通过远程接口发送
		return iface.GetNode().Remote().Send(message)
	}
}

// Call 发送消息到指定进程
func (s *System) Call(message *iface.ActorMessage, timeout time.Duration) *iface.Response {
	if s.shuttingDown.Load() {
		return iface.NewErrorResponse(errs.ErrSystemShuttingDown)
	}
	to := message.GetTo()
	if s.isLocalPid(to) {
		// 本地消息，直接发送到本地进程
		process := s.GetProcess(to)
		if process == nil {
			return iface.NewErrorResponse(errs.ErrProcessNotFound)
		}
		return process.Call(message, timeout)
	} else {
		// 远程消息，通过远程接口发送
		return iface.GetNode().Remote().Call(message, timeout)
	}

}

func (s *System) PushTask(to *iface.Pid, f iface.Task) error {
	if s.shuttingDown.Load() {
		return errs.ErrSystemShuttingDown
	}
	process := s.GetProcess(to)
	if process == nil {
		return errs.ErrProcessNotFound
	}
	return process.PushTask(f)
}

func (s *System) PushTaskAndWait(to *iface.Pid, timeout time.Duration, task iface.Task) error {
	if s.shuttingDown.Load() {
		return errs.ErrSystemShuttingDown
	}
	process := s.GetProcess(to)
	if process == nil {
		return errs.ErrProcessNotFound
	}
	return process.PushTaskAndWait(timeout, task)
}

func (s *System) Select(name string, strategy iface.RouteStrategy) *iface.Pid {
	if process := s.GetProcessByName(name); process != nil {
		return process.Context().ID()
	}
	return iface.GetNode().Remote().Select(name, strategy)
}

func (s *System) CastPid(to interface{}) *iface.Pid {
	var pid *iface.Pid
	switch _to := to.(type) {
	case string:
		return s.Select(_to, iface.RouteRandom)
	case *iface.Pid:
		return pid
	default:
		return nil
	}
}

// Shutdown 优雅关闭 Actor 系统
func (s *System) Shutdown(timeout time.Duration) error {
	// 标记为关闭状态，拒绝新的消息和进程创建
	if !s.shuttingDown.CompareAndSwap(false, true) {
		return nil // 已经在关闭中
	}

	processes := s.GetAllProcesses()
	for _, process := range processes {
		if err := process.Shutdown(); err != nil {
			continue
		}
	}
	return nil
}
