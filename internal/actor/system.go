/**
 * @Author: dingQingHui
 * @Description:
 * @File: system
 * @Version: 1.0.0
 * @Date: 2023/12/7 14:54
 */

package actor

import (
	"errors"
	"fmt"
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

	pid := s.newPid()
	mailBox := NewMailbox()
	// 先创建 process，传入 nil context（稍后设置）
	process := NewProcess(nil, mailBox)
	// 创建 context 配置
	cfg := &contextConfig{
		pid:     pid,
		actor:   actor,
		system:  s,
		process: process,
	}
	// 创建 context
	context := newBaseActorContext(cfg)
	// 设置 process 的 context
	process.(*Process).setContext(context)
	// 注册 mailbox 的 handlers
	mailBox.RegisterHandlers(context, NewDefaultDispatcher(1024))

	s.registerProcess(pid, process)

	_ = process.PushTask(func(ctx iface.IContext) error {
		return ctx.Actor().OnInit(ctx, args)
	})

	return pid
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

// Send 发送消息到指定进程
func (s *System) Send(message *iface.Message) error {
	if s.shuttingDown.Load() {
		return errors.New("system is shutting down")
	}
	to := message.To
	if to.GetNodeId() == s.GetNode().GetId() {
		process := s.GetProcess(message.GetTo())
		if process == nil {
			return errors.New("process not found")
		}
		return process.PushMessage(message)
	} else {
		return s.GetNode().GetRemote().Send(message)
	}
}

// Request 发送消息到指定进程
func (s *System) Request(message *iface.Message, reply interface{}) error {
	if s.shuttingDown.Load() {
		return fmt.Errorf("system is shutting down")
	}

	to := message.To

	var response *iface.RespondMessage
	if to.GetNodeId() == s.GetNode().GetId() {
		response = s.localRequest(message)
	} else {
		response = s.GetNode().GetRemote().Request(message, time.Second*3)
	}
	if errMsg := response.GetError(); errMsg != "" {
		return fmt.Errorf("request failed: %s", errMsg)
	}
	if data := response.GetData(); len(data) > 0 {
		if err := s.GetSerializer().Unmarshal(data, reply); err != nil {
			return fmt.Errorf("unmarshal reply failed: %w", err)
		}
	}
	return nil
}

func (s *System) localRequest(message *iface.Message) *iface.RespondMessage {
	waiter := newChanWaiter[*iface.RespondMessage](time.Second * 3)
	msg := &iface.SyncMessage{Message: message}
	msg.SetResponse(func(message *iface.RespondMessage) {
		waiter.Done(message)
	})

	process := s.GetProcess(msg.GetTo())
	if process == nil {
		return iface.NewErrorResponse("process not found")
	}
	if err := process.PushMessage(msg); err != nil {
		return iface.NewErrorResponse(err.Error())
	}

	res, err := waiter.Wait()
	if err != nil {
		return iface.NewErrorResponse(err.Error())
	}
	return res
}

func (s *System) PushTask(pid *iface.Pid, f iface.Task) error {
	if s.shuttingDown.Load() {
		return errors.New("system is shutting down")
	}
	process := s.GetProcess(pid)
	if process == nil {
		return errors.New("process not found")
	}
	return process.PushTask(f)
}

func (s *System) PushTaskAndWait(pid *iface.Pid, timeout time.Duration, task iface.Task) error {
	if s.shuttingDown.Load() {
		return errors.New("system is shutting down")
	}
	process := s.GetProcess(pid)
	if process == nil {
		return errors.New("process not found")
	}
	return process.PushTaskAndWait(timeout, task)
}

func (s *System) PushMessage(pid *iface.Pid, message interface{}) error {
	if s.shuttingDown.Load() {
		return errors.New("system is shutting down")
	}
	process := s.GetProcess(pid)
	if process == nil {
		return errors.New("process not found")
	}
	return process.PushMessage(message)
}

func (s *System) Select(name string, strategy iface.RouteStrategy) (*iface.Pid, error) {
	// 查找本地名字
	if process := s.GetProcessByName(name); process != nil {
		return process.Context().ID(), nil
	}
	// 本地找不到通过remote查找
	node := s.GetNode()
	if node == nil {
		return nil, errors.New("node is nil")
	}
	remote := node.GetRemote()
	if remote == nil {
		return nil, errors.New("remote is nil")
	}
	return remote.Select(name, strategy)
}

func (s *System) RegisterName(pid *iface.Pid, process iface.IProcess, name string, isGlobal bool) error {
	if len(name) == 0 {
		return fmt.Errorf("name cannot be empty")
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
			return fmt.Errorf("node is not initialized")
		}
		remote := s.GetNode().GetRemote()
		if remote == nil {
			return fmt.Errorf("remote is not initialized")
		}
		if err := remote.RegistryName(name); err != nil {
			// 如果远程注册失败，回滚本地注册
			s.nameDict.Delete(name)
			pid.Name = oldName
			if oldName != "" {
				s.nameDict.Set(oldName, process)
			}
			return fmt.Errorf("remote registry name failed: %w", err)
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
		if err := process.Exit(); err != nil {
			continue
		}
	}
	return nil
}
