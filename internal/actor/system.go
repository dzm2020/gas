package actor

import (
	"gas/internal/errs"
	"gas/internal/iface"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/lib"
	"sync/atomic"
	"time"

	"github.com/duke-git/lancet/v2/maputil"
)

// System 管理本地 Actor 进程系统
type System struct {
	uniqId       atomic.Uint64
	processDict  *maputil.ConcurrentMap[uint64, iface.IProcess] // ID到进程的映射
	nameDict     *maputil.ConcurrentMap[string, *iface.Pid]     // 名字到进程ID的映射
	shuttingDown atomic.Bool
}

// NewSystem 创建新的 Actor 系统实例
func NewSystem() *System {
	return &System{
		uniqId:      atomic.Uint64{},
		processDict: maputil.NewConcurrentMap[uint64, iface.IProcess](10),
		nameDict:    maputil.NewConcurrentMap[string, *iface.Pid](10),
	}
}

// ==================== 进程管理 ====================

// Spawn 创建新的 Actor 进程
func (s *System) Spawn(actor iface.IActor, args ...interface{}) *iface.Pid {
	node := iface.GetNode()
	pid := iface.NewPid(node.GetID(), s.uniqId.Add(1))

	ctx := &actorContext{
		process: nil,
		pid:     pid,
		actor:   actor,
		router:  GetRouterForActor(actor),
	}

	mailBox := NewMailbox()
	process := NewProcess(ctx, mailBox)
	ctx.process = process

	mailBox.RegisterHandlers(ctx, NewDefaultDispatcher(1024))
	s.Add(pid, process)

	_ = s.SubmitTask(pid, func(ctx iface.IContext) error {
		return ctx.Actor().OnInit(ctx, args)
	})

	return pid
}

// Add 注册进程到系统中
func (s *System) Add(pid *iface.Pid, process iface.IProcess) {
	s.processDict.Set(pid.GetServiceId(), process)
	if pid.GetName() != "" {
		_ = s.Named(pid.GetName(), pid)
	}
}

// Remove 从系统中移除进程
func (s *System) Remove(pid *iface.Pid) {
	s.processDict.Delete(pid.GetServiceId())
	s.Unname(pid)
}

// GetProcess 根据标识符获取进程，支持 string 名字或 *iface.Pid
func (s *System) GetProcess(to interface{}) iface.IProcess {
	if to == nil {
		return nil
	}

	switch v := to.(type) {
	case string:
		return s.GetProcessByName(v)
	case *iface.Pid:
		if v.GetServiceId() > 0 {
			return s.GetProcessById(v.GetServiceId())
		}
		if v.GetName() != "" {
			return s.GetProcessByName(v.GetName())
		}
	}

	return nil
}

// GetProcessById 根据服务ID获取进程
func (s *System) GetProcessById(id uint64) iface.IProcess {
	process, _ := s.processDict.Get(id)
	return process
}

// GetProcessByName 根据名字获取进程
func (s *System) GetProcessByName(name string) iface.IProcess {
	if name == "" {
		return nil
	}
	pid, exists := s.nameDict.Get(name)
	if !exists || pid == nil {
		return nil
	}
	return s.GetProcessById(pid.GetServiceId())
}

// GetAllProcesses 获取系统中所有进程
func (s *System) GetAllProcesses() []iface.IProcess {
	var processes []iface.IProcess
	s.processDict.Range(func(_ uint64, value iface.IProcess) bool {
		processes = append(processes, value)
		return true
	})
	return processes
}

// ==================== 名字管理 ====================

// Named 为进程注册名字
func (s *System) Named(name string, pid *iface.Pid) error {
	if len(name) == 0 {
		return errs.ErrNameCannotBeEmpty()
	}
	// 检查：如果进程已经有名字，不允许修改
	if pid.GetName() != "" {
		return errs.ErrNameChangeNotAllowed()
	}
	// 检查：名字是否已被其他进程注册
	if s.HasName(name) {
		return errs.ErrNameAlreadyRegistered(name)
	}
	// 注册名字：更新 pid 的名称
	pid.Name = name
	// 在本地注册新名称
	s.nameDict.Set(name, pid)
	// 全局名字注册
	node := iface.GetNode()
	cluster := node.Cluster()
	if pid.IsGlobalName() {
		node.AddTag(name)
		_ = cluster.UpdateMember()
	}
	return nil
}

// HasName 检查名字是否已注册
func (s *System) HasName(name string) bool {
	_, exists := s.nameDict.Get(name)
	return exists
}

// Unname 注销进程的名字
func (s *System) Unname(pid *iface.Pid) {
	if pid.GetName() == "" {
		return
	}

	name := pid.GetName()
	s.nameDict.Delete(name)

	if pid.IsGlobalName() {
		node := iface.GetNode()
		cluster := node.Cluster()
		node.RemoteTag(name)
		_ = cluster.UpdateMember()
	}
}

// ==================== 消息发送 ====================

// Call 同步调用 Actor，等待响应
func (s *System) Call(message *iface.ActorMessage) ([]byte, error) {
	if message.GetTo().IsLocal() {
		return s.localCall(message)
	}
	node := iface.GetNode()
	cluster := node.Cluster()
	return cluster.Call(message)
}

// Send 异步发送消息给 Actor
func (s *System) Send(message *iface.ActorMessage) error {
	if message.GetTo().IsLocal() {
		return s.localSend(message)
	}
	node := iface.GetNode()
	cluster := node.Cluster()
	return cluster.Send(message)
}

// localCall 本地同步调用
func (s *System) localCall(message *iface.ActorMessage) (data []byte, err error) {
	waiter := lib.NewChanWaiter(message.GetDeadline())
	message.SetResponse(func(bin []byte, e error) {
		data = bin
		err = e
	})

	if err = s.sendToProcess(message.To, message); err != nil {
		waiter.Done()
		return
	}
	err = waiter.Wait()
	return
}

// localSend 本地异步发送
func (s *System) localSend(message *iface.ActorMessage) error {
	return s.sendToProcess(message.To, message)
}

// ==================== 任务提交 ====================

// SubmitTask 提交异步任务到指定进程
func (s *System) SubmitTask(to interface{}, task iface.Task) error {
	if err := s.checkShuttingDown(); err != nil {
		return err
	}
	msg := iface.NewTaskMessage(task)
	return s.sendToProcess(to, msg)
}

// SubmitTaskAndWait 提交同步任务到指定进程，等待执行完成
func (s *System) SubmitTaskAndWait(to interface{}, task iface.Task, timeout time.Duration) error {
	deadline := time.Now().Add(timeout).Unix()
	waiter := lib.NewChanWaiter(deadline)

	var err error
	syncTask := func(ctx iface.IContext) error {
		if task != nil {
			err = task(ctx)
		}
		waiter.Done()
		return err
	}

	msg := iface.NewTaskMessage(syncTask)
	if err = s.sendToProcess(to, msg); err != nil {
		waiter.Done()
		return err
	}

	err = waiter.Wait()
	return err
}

// ==================== 辅助方法 ====================

// GenPid 根据标识符和路由策略生成进程ID，优先返回本地进程ID
func (s *System) GenPid(to interface{}, strategy discovery.RouteStrategy) *iface.Pid {
	// 优先本地
	process := s.GetProcess(to)
	if process != nil {
		return process.Context().ID()
	}
	// 选择远程
	switch v := to.(type) {
	case string:
		node := iface.GetNode()
		cluster := node.Cluster()
		nodeId := cluster.Select(v, strategy)
		return iface.NewPidWithName(v, nodeId)
	}
	return nil
}

// sendToProcess 发送消息到指定进程
func (s *System) sendToProcess(to, msg interface{}) error {
	if err := s.checkShuttingDown(); err != nil {
		return err
	}

	process := s.GetProcess(to)
	if process == nil {
		return errs.ErrProcessNotFound
	}
	return process.Input(msg)
}

// checkShuttingDown 检查系统是否正在关闭
func (s *System) checkShuttingDown() error {
	if s.shuttingDown.Load() {
		return errs.ErrSystemShuttingDown
	}
	return nil
}

// ==================== 系统关闭 ====================

// Shutdown 优雅关闭 Actor 系统
// timeout 参数预留用于未来实现超时控制
func (s *System) Shutdown(timeout time.Duration) error {
	// 标记为关闭状态，拒绝新的消息和进程创建
	if !s.shuttingDown.CompareAndSwap(false, true) {
		return nil // 已经在关闭中
	}

	processes := s.GetAllProcesses()
	for _, process := range processes {
		_ = process.Shutdown()
	}
	return nil
}
