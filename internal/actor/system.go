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
	"gas/internal/iface"
	"gas/pkg/utils/serializer"
	"sync/atomic"
	"time"

	"github.com/duke-git/lancet/v2/maputil"
)

func loadOptions(options ...iface.Option) *iface.Options {
	opts := &iface.Options{}
	for _, option := range options {
		option(opts)
	}
	return opts
}

// System Actor 系统，管理所有进程和消息传递
type System struct {
	uniqId       atomic.Uint64
	nameDict     *maputil.ConcurrentMap[string, iface.IProcess]
	processDict  *maputil.ConcurrentMap[uint64, iface.IProcess]
	serializer   serializer.ISerializer
	node         iface.INode
	shuttingDown atomic.Bool
}

// NewSystem 创建新的 Actor 系统
func NewSystem() *System {
	return &System{
		uniqId:      atomic.Uint64{},
		nameDict:    maputil.NewConcurrentMap[string, iface.IProcess](10),
		processDict: maputil.NewConcurrentMap[uint64, iface.IProcess](10),
		serializer:  serializer.Json,
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
func (s *System) GetSerializer() serializer.ISerializer {
	return s.serializer
}

// SetSerializer 设置序列化器
func (s *System) SetSerializer(ser serializer.ISerializer) {
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
func (s *System) Spawn(actor iface.IActor, options ...iface.Option) (*iface.Pid, iface.IProcess) {
	if s.shuttingDown.Load() {
		return nil, nil
	}
	opts := loadOptions(options...)
	pid := s.newPid()
	pid.Name = opts.Name

	mailBox := NewMailbox()
	// 先创建 process，传入 nil context（稍后设置）
	process := NewProcess(nil, mailBox)
	// 创建 context 配置
	cfg := &contextConfig{
		pid:         pid,
		actor:       actor,
		middlewares: opts.Middlewares,
		router:      opts.Router,
		system:      s,
		process:     process,
	}
	// 创建 context
	context := newBaseActorContext(cfg)
	// 设置 process 的 context
	process.(*Process).setContext(context)
	// 注册 mailbox 的 handlers
	mailBox.RegisterHandlers(context, NewDefaultDispatcher(1024))

	s.registerProcess(pid, process)

	_ = process.PushTask(func(ctx iface.IContext) error {
		return ctx.Actor().OnInit(ctx, opts.Params)
	})

	return pid, process
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
	process := s.GetProcess(message.GetTo())
	if process == nil {
		return errors.New("process not found")
	}
	return process.PushMessage(message)
}

// Request 发送消息到指定进程
func (s *System) Request(message *iface.Message, timeout time.Duration) *iface.RespondMessage {
	if s.shuttingDown.Load() {
		return iface.NewErrorResponse("system is shutting down")
	}
	waiter := newChanWaiter[*iface.RespondMessage](timeout)

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
	deadline := time.Now().Add(timeout)

	// 获取所有进程的快照
	processes := s.GetAllProcesses()

	// 第一步：依次调用每个进程的 Exit()
	// Exit() 会推送退出任务到队列，并等待最多1秒
	for _, process := range processes {
		if err := process.Exit(); err != nil {
			continue
		}
	}

	// 第二步：等待所有进程从系统中注销（进程数量变为0）
	if err := s.waitForAllProcessesExited(time.Until(deadline)); err != nil {
		return err
	}

	return nil
}

// waitForAllProcessesExited 等待所有进程从系统中注销（进程数量变为0）
func (s *System) waitForAllProcessesExited(timeout time.Duration) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	for {
		// 检查进程数量是否为0
		processCount := s.getProcessCount()
		if processCount == 0 {
			return nil
		}

		select {
		case <-ticker.C:
			continue
		case <-timeoutTimer.C:
			return errors.New("shutdown timeout: some processes are not exited")
		}
	}
}

// getProcessCount 获取当前系统中的进程数量
func (s *System) getProcessCount() int {
	count := 0
	s.processDict.Range(func(key uint64, value iface.IProcess) bool {
		count++
		return true
	})
	return count
}
