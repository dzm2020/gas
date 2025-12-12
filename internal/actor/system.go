// Package actor 提供 Actor 模型实现，包括进程管理、消息路由、定时器等核心功能
package actor

import (
	"gas/internal/errs"
	"gas/internal/iface"
	"gas/pkg/lib"
	"sync/atomic"
	"time"
)

// System Actor 系统，管理所有进程和消息传递
type System struct {
	*Manager     // 名字管理器
	uniqId       atomic.Uint64
	serializer   lib.ISerializer
	node         iface.INode
	shuttingDown atomic.Bool
}

// NewSystem 创建新的 Actor 系统
func NewSystem() *System {
	sys := &System{
		uniqId:     atomic.Uint64{},
		Manager:    NewNameManager(),
		serializer: lib.Json,
	}
	return sys
}

// SetNode 设置节点实例
func (s *System) SetNode(n iface.INode) {
	s.node = n
	s.Manager.SetNode(n)
}

// GetNode 获取节点实例
func (s *System) GetNode() iface.INode {
	return s.node
}

// GetSerializer 获取序列化器
func (s *System) GetSerializer() lib.ISerializer {
	return s.serializer
}

// SetSerializer 设置序列化器
func (s *System) SetSerializer(ser lib.ISerializer) {
	s.serializer = ser
}

// newPid 创建新的进程ID
func (s *System) newPid() *iface.Pid {
	pid := &iface.Pid{
		ServiceId: s.uniqId.Add(1),
	}
	if s.node != nil {
		pid.NodeId = s.node.GetID()
	}
	return pid
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
	ctx.system = s
	mailBox.RegisterHandlers(ctx, NewDefaultDispatcher(1024))
	s.Add(pid, process)
	_ = process.PushTask(func(ctx iface.IContext) error {
		return ctx.Actor().OnInit(ctx, args)
	})
	return pid
}

// getProcessChecked 获取进程并检查系统状态
func (s *System) getProcessChecked(pid *iface.Pid) (iface.IProcess, error) {
	if err := s.checkShuttingDown(); err != nil {
		return nil, err
	}
	process := s.GetProcess(pid)
	if process == nil {
		return nil, errs.ErrProcessNotFound
	}
	return process, nil
}

// checkShuttingDown 检查系统是否正在关闭
func (s *System) checkShuttingDown() error {
	if s.shuttingDown.Load() {
		return errs.ErrSystemShuttingDown
	}
	return nil
}

// getRemote 获取远程通信接口，并进行空指针检测
func (s *System) getRemote() (iface.IRemote, error) {
	node := s.GetNode()
	if node == nil {
		return nil, errs.ErrNodeIsNil
	}
	remote := node.GetRemote()
	if remote == nil {
		return nil, errs.ErrRemoteIsNil
	}
	return remote, nil
}

// isLocalPid 判断进程 ID 是否为本地进程
func (s *System) isLocalPid(pid *iface.Pid) bool {
	if pid == nil {
		return false
	}
	node := s.GetNode()
	if node == nil {
		return false
	}
	return pid.GetNodeId() == node.GetID()
}

// Send 发送消息到指定进程
func (s *System) Send(message *iface.Message) error {
	if err := s.checkShuttingDown(); err != nil {
		return err
	}
	if message == nil {
		return errs.ErrMessageIsNilInSystem()
	}
	to := message.GetTo()
	if s.isLocalPid(to) {
		// 本地消息，直接发送到本地进程
		process, err := s.getProcessChecked(to)
		if err != nil {
			return err
		}
		return process.Send(message)
	}

	// 远程消息，通过远程接口发送
	remote, err := s.getRemote()
	if err != nil {
		return err
	}
	return remote.Send(message)
}

// Call 发送消息到指定进程
func (s *System) Call(message *iface.Message, timeout time.Duration) *iface.Response {
	if s.shuttingDown.Load() {
		return iface.NewErrorResponse(errs.ErrSystemShuttingDown)
	}

	if message == nil {
		return iface.NewErrorResponse(errs.ErrMessageIsNilInMsg)
	}

	to := message.GetTo()
	if s.isLocalPid(to) {
		// 本地消息，直接发送到本地进程
		process, err := s.getProcessChecked(to)
		if err != nil {
			return iface.NewErrorResponse(err)
		}
		return process.Call(message, timeout)
	}

	// 远程消息，通过远程接口发送
	remote, err := s.getRemote()
	if err != nil {
		return iface.NewErrorResponse(err)
	}
	return remote.Call(message, timeout)
}

func (s *System) PushTask(pid *iface.Pid, f iface.Task) error {
	process, err := s.getProcessChecked(pid)
	if err != nil {
		return err
	}
	return process.PushTask(f)
}

func (s *System) PushTaskAndWait(pid *iface.Pid, timeout time.Duration, task iface.Task) error {
	process, err := s.getProcessChecked(pid)
	if err != nil {
		return err
	}
	return process.PushTaskAndWait(timeout, task)
}

func (s *System) Select(name string, strategy iface.RouteStrategy) *iface.Pid {
	if process := s.GetProcessByName(name); process != nil {
		return process.Context().ID()
	}
	remote, err := s.getRemote()
	if err != nil {
		return nil
	}
	return remote.Select(name, strategy)
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
