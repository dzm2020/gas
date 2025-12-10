// Package actor 提供 Actor 模型实现，包括进程管理、消息路由、定时器等核心功能
package actor

import (
	"gas/internal/iface"
	"gas/pkg/lib"
	"sync/atomic"
	"time"

	"github.com/duke-git/lancet/v2/maputil"
)

// System Actor 系统，管理所有进程和消息传递
type System struct {
	uniqId       atomic.Uint64
	nameDict     *maputil.ConcurrentMap[string, iface.IProcess]
	processDict  *maputil.ConcurrentMap[uint64, iface.IProcess]
	serializer   lib.ISerializer
	node         iface.INode
	shuttingDown atomic.Bool
}

// NewSystem 创建新的 Actor 系统
func NewSystem() *System {
	return &System{
		uniqId:      atomic.Uint64{},
		nameDict:    maputil.NewConcurrentMap[string, iface.IProcess](10),
		processDict: maputil.NewConcurrentMap[uint64, iface.IProcess](10),
		serializer:  lib.Json,
	}
}

// SetNode 设置节点实例
func (s *System) SetNode(n iface.INode) {
	s.node = n
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
		pid.NodeId = s.node.GetId()
	}
	return pid
}

// Spawn 创建新的 actor 进程
func (s *System) Spawn(actor iface.IActor, args ...interface{}) *iface.Pid {
	if s.shuttingDown.Load() {
		return nil
	}
	ctx := newActorContext()
	pid := s.newPid()
	mailBox := NewMailbox()
	process := NewProcess(ctx, mailBox)

	ctx.process = process
	ctx.pid = pid
	ctx.actor = actor
	ctx.system = s

	mailBox.RegisterHandlers(ctx, NewDefaultDispatcher(1024))

	s.registerProcess(pid, process)

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
		return nil, ErrProcessNotFound
	}
	return process, nil
}

// GetProcess 根据 Pid 获取进程
func (s *System) GetProcess(pid *iface.Pid) iface.IProcess {
	if pid == nil {
		return nil
	}
	if name := pid.GetName(); name != "" {
		p, _ := s.nameDict.Get(name)
		return p
	}
	if id := pid.GetServiceId(); id > 0 {
		p, _ := s.processDict.Get(id)
		return p
	}
	return nil
}

// GetProcessById 根据 ID 获取进程
func (s *System) GetProcessById(id uint64) iface.IProcess {
	process, _ := s.processDict.Get(id)
	return process
}

// GetProcessByName 根据名称获取进程
func (s *System) GetProcessByName(name string) iface.IProcess {
	process, _ := s.nameDict.Get(name)
	return process
}

// checkShuttingDown 检查系统是否正在关闭
func (s *System) checkShuttingDown() error {
	if s.shuttingDown.Load() {
		return ErrSystemShuttingDown
	}
	return nil
}

// getRemote 获取远程通信接口，并进行空指针检测
func (s *System) getRemote() (iface.IRemote, error) {
	node := s.GetNode()
	if node == nil {
		return nil, ErrNodeIsNil
	}
	remote := node.GetRemote()
	if remote == nil {
		return nil, ErrRemoteIsNil
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
	return pid.GetNodeId() == node.GetId()
}

// Send 发送消息到指定进程
func (s *System) Send(message *iface.Message) error {
	if err := s.checkShuttingDown(); err != nil {
		return err
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
		return iface.NewErrorResponse("system is shutting down")
	}

	if message == nil {
		return iface.NewErrorResponse("message is nil")
	}

	to := message.GetTo()
	if s.isLocalPid(to) {
		// 本地消息，直接发送到本地进程
		process, err := s.getProcessChecked(to)
		if err != nil {
			return iface.NewErrorResponse(err.Error())
		}
		return process.Call(message, timeout)
	}

	// 远程消息，通过远程接口发送
	remote, err := s.getRemote()
	if err != nil {
		return iface.NewErrorResponse(err.Error())
	}
	return remote.Request(message, timeout)
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

func (s *System) Select(name string, strategy iface.RouteStrategy) (*iface.Pid, error) {
	// 查找本地名字
	if process := s.GetProcessByName(name); process != nil {
		return process.Context().ID(), nil
	}

	// 本地找不到通过remote查找
	// 远程消息，通过远程接口发送
	remote, err := s.getRemote()
	if err != nil {
		return nil, err
	}
	return remote.Select(name, strategy)
}

func (s *System) RegisterName(pid *iface.Pid, process iface.IProcess, name string, isGlobal bool) error {
	if len(name) == 0 {
		return ErrNameCannotBeEmpty()
	}

	// 本地注册：更新 pid 的名称并在 system 中注册
	oldName := pid.Name
	pid.Name = name

	// 如果之前有名称，先从 nameDict 中删除旧名称
	if oldName != "" && oldName != name {
		s.nameDict.Delete(oldName)
	}

	// 在本地 system 中注册新名称
	s.nameDict.Set(name, process)

	// 如果是全局注册，还需要在远程注册
	if isGlobal {
		if s.GetNode() == nil {
			return ErrNodeNotInitialized()
		}
		remote := s.GetNode().GetRemote()
		if remote == nil {
			return ErrRemoteNotInitialized()
		}
		if err := remote.RegistryName(name); err != nil {
			// 如果远程注册失败，回滚本地注册
			s.nameDict.Delete(name)
			pid.Name = oldName
			if oldName != "" {
				s.nameDict.Set(oldName, process)
			}
			return ErrRemoteRegistryNameFailed(err)
		}
	}
	return nil
}

// registerProcess 注册进程
func (s *System) registerProcess(pid *iface.Pid, process iface.IProcess) {
	s.processDict.Set(pid.GetServiceId(), process)
	if pid.GetName() != "" {
		s.nameDict.Set(pid.GetName(), process)
	}
}

// unregisterProcess 注销进程
func (s *System) unregisterProcess(pid *iface.Pid) {
	s.processDict.Delete(pid.GetServiceId())
	if pid.GetName() != "" {
		s.nameDict.Delete(pid.GetName())
	}
}

// GetAllProcesses 获取所有进程
func (s *System) GetAllProcesses() []iface.IProcess {
	var processes []iface.IProcess
	s.processDict.Range(func(_ uint64, value iface.IProcess) bool {
		processes = append(processes, value)
		return true
	})
	return processes
}

// Shutdown 优雅关闭 Actor 系统
// timeout: 最大等待时间，如果为 0 则使用默认值 10 秒
func (s *System) Shutdown(timeout time.Duration) error {
	// 标记为关闭状态，拒绝新的消息和进程创建
	if !s.shuttingDown.CompareAndSwap(false, true) {
		return nil // 已经在关闭中
	}

	if timeout == 0 {
		timeout = 10 * time.Second
	}
	// 获取所有进程的快照
	processes := s.GetAllProcesses()

	// 第一步：依次调用每个进程的 Exit()
	// Exit() 会推送退出任务到队列，并等待最多1秒
	for _, process := range processes {
		if err := process.Shutdown(); err != nil {
			continue
		}
	}
	return nil
}
