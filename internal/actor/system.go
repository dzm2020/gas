package actor

import (
	"errors"
	"gas/internal/iface"
	discovery "gas/pkg/discovery/iface"
	"gas/pkg/glog"
	"gas/pkg/lib"
	"gas/pkg/lib/xerror"
	"sync/atomic"
	"time"

	"github.com/duke-git/lancet/v2/maputil"
	"go.uber.org/zap"
)

var (
	ErrProcessExiting        = errors.New("进程正在退出")
	ErrProcessNotFound       = errors.New("进程未找到")
	ErrMessageIsNil          = errors.New("消息为空")
	ErrSystemShuttingDown    = errors.New("系统正在关闭")
	ErrNameCannotBeEmpty     = errors.New("名字不能为空")
	ErrNameChangeNotAllowed  = errors.New("不允许重复命名")
	ErrNameAlreadyRegistered = errors.New("名字已注册")
	ErrClusterIsNil          = errors.New("集群组件未初始化")
)

const (
	// DefaultDispatcherThroughput 默认调度器吞吐量
	DefaultDispatcherThroughput = 1024
)

var _ iface.ISystem = (*System)(nil)

type System struct {
	uniqId       atomic.Uint64
	processDict  *maputil.ConcurrentMap[uint64, iface.IProcess] // ID到进程的映射
	nameDict     *maputil.ConcurrentMap[string, *iface.Pid]     // 名字到进程ID的映射
	shuttingDown atomic.Bool
	node         iface.INode
}

func NewSystem(node iface.INode) *System {
	return &System{
		node:        node,
		uniqId:      atomic.Uint64{},
		processDict: maputil.NewConcurrentMap[uint64, iface.IProcess](10),
		nameDict:    maputil.NewConcurrentMap[string, *iface.Pid](10),
	}
}

// ==================== 进程管理 ====================

// Spawn 创建新的 Actor 进程
func (s *System) Spawn(actor iface.IActor, args ...interface{}) *iface.Pid {
	node := s.node
	pid := iface.NewPid(node.GetID(), s.uniqId.Add(1))

	ctx := &actorContext{
		process: nil,
		pid:     pid,
		actor:   actor,
		router:  GetRouterForActor(actor),
		node:    node,
		system:  s,
		timeout: DefaultCallTimeout,
	}

	mailBox := NewMailbox()
	process := NewProcess(ctx, mailBox)
	ctx.process = process

	mailBox.RegisterHandlers(ctx, NewDefaultDispatcher(DefaultDispatcherThroughput))
	s.Add(pid, process)

	// 提交初始化任务，如果失败则记录日志但不影响进程创建
	if err := s.SubmitTask(pid, func(ctx iface.IContext) error {
		return ctx.Actor().OnInit(ctx, args)
	}); err != nil {
		glog.Error("提交Actor初始化任务失败", zap.Any("pid", pid), zap.Error(err))
	}

	return pid
}

// Add 注册进程到系统中
func (s *System) Add(pid *iface.Pid, process iface.IProcess) {
	s.processDict.Set(pid.GetServiceId(), process)
}

// Remove 从系统中移除进程
func (s *System) Remove(pid *iface.Pid) error {
	s.processDict.Delete(pid.GetServiceId())
	// 尝试取消命名，失败不影响移除操作
	return s.Unname(pid)
}

func (s *System) GetProcess(to *iface.Pid) iface.IProcess {
	if to == nil {
		return nil
	}
	if to.GetServiceId() > 0 {
		return s.GetProcessById(to.GetServiceId())
	}
	if to.GetName() != "" {
		return s.GetProcessByName(to.GetName())
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
		return ErrNameCannotBeEmpty
	}

	if pid.GetName() != "" {
		return ErrNameChangeNotAllowed
	}

	if s.HasName(name) {
		return ErrNameAlreadyRegistered
	}

	pid.Name = name
	s.nameDict.Set(name, pid)

	if !pid.IsGlobalName() {
		return nil
	}

	return s.clusterNamed(name)
}

func (s *System) clusterNamed(name string) error {
	cluster := s.node.Cluster()
	if cluster == nil {
		return ErrClusterIsNil
	}
	s.node.AddTag(name)
	if err := cluster.UpdateMember(); err != nil {
		return err
	}
	return nil
}

// HasName 检查名字是否已注册
func (s *System) HasName(name string) bool {
	_, exists := s.nameDict.Get(name)
	return exists
}

// Unname 注销进程的名字
func (s *System) Unname(pid *iface.Pid) error {
	if pid.GetName() == "" {
		return nil
	}

	name := pid.GetName()
	s.nameDict.Delete(name)

	if !pid.IsGlobalName() {
		return nil
	}

	return s.clusterNamed(name)
}

func (s *System) clusterUnname(name string) error {
	cluster := s.node.Cluster()
	if cluster == nil {
		return ErrClusterIsNil
	}
	s.node.RemoteTag(name)
	if err := cluster.UpdateMember(); err != nil {
		return err
	}
	return nil
}

// ==================== 消息发送 ====================

func (s *System) isLocalMessage(message *iface.ActorMessage) bool {
	return message.GetTo().GetNodeId() == s.node.GetID()
}

// Send 异步发送消息给 Actor
func (s *System) Send(message *iface.ActorMessage) error {
	if s.isLocalMessage(message) {
		return s.localSend(message)
	}
	cluster := s.node.Cluster()
	if cluster == nil {
		return ErrClusterIsNil
	}
	if err := cluster.Send(message); err != nil {
		return err
	}
	return nil
}

// Call 同步调用 Actor，等待响应
func (s *System) Call(message *iface.ActorMessage) ([]byte, error) {
	if s.isLocalMessage(message) {
		return s.localCall(message)
	}
	cluster := s.node.Cluster()
	if cluster == nil {
		return nil, ErrClusterIsNil
	}
	data, err := cluster.Call(message)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// localCall 本地同步调用
func (s *System) localCall(message *iface.ActorMessage) (data []byte, err error) {
	timeout := lib.NowDelay(message.GetDeadline(), 0)
	waiter := lib.NewChanWaiter[[]byte](timeout)
	message.SetResponse(func(bin []byte, e error) {
		waiter.Done(bin, e)
	})
	if err = s.sendToProcess(message.To, message); err != nil {
		waiter.Done(nil, err)
		return
	}
	data, err = waiter.Wait()
	return
}

// localSend 本地异步发送
func (s *System) localSend(message *iface.ActorMessage) error {
	return s.sendToProcess(message.To, message)
}

// ==================== 任务提交 ====================

// SubmitTask 提交异步任务到指定进程
func (s *System) SubmitTask(to *iface.Pid, task iface.Task) error {
	msg := iface.NewTaskMessage(task)
	return s.sendToProcess(to, msg)
}

// SubmitTaskAndWait 提交同步任务到指定进程，等待执行完成
func (s *System) SubmitTaskAndWait(to *iface.Pid, task iface.Task, timeout time.Duration) (err error) {
	waiter := lib.NewChanWaiter[[]byte](timeout)

	syncTask := func(ctx iface.IContext) error {
		taskErr := task(ctx)
		waiter.Done(nil, taskErr)
		return taskErr
	}

	msg := iface.NewTaskMessage(syncTask)
	if err = s.sendToProcess(to, msg); err != nil {
		waiter.Done(nil, err)
		return err
	}

	_, err = waiter.Wait()
	return
}

// ==================== 辅助方法 ====================

// sendToProcess 发送消息到指定进程
func (s *System) sendToProcess(to *iface.Pid, msg iface.IMessage) error {
	if err := s.checkShuttingDown(); err != nil {
		return err
	}
	process := s.GetProcess(to)
	if process == nil {
		return xerror.Wrapf(ErrProcessNotFound, "pid=%v", to)
	}
	if err := process.PostMessage(msg); err != nil {
		return xerror.Wrapf(err, "发送消息到进程失败 (pid=%v)", to)
	}
	return nil
}

func (s *System) Select(name string, strategy discovery.RouteStrategy) *iface.Pid {
	process := s.GetProcessByName(name)
	if process != nil {
		return process.Context().ID()
	}
	cluster := s.node.Cluster()
	nodeId := cluster.Select(name, strategy)

	return &iface.Pid{
		NodeId:    nodeId,
		Name:      name,
		ServiceId: 0,
	}
}

// ==================== 系统关闭 ====================

// checkShuttingDown 检查系统是否正在关闭
func (s *System) checkShuttingDown() error {
	if s.shuttingDown.Load() {
		return ErrSystemShuttingDown
	}
	return nil
}

// Shutdown 优雅关闭 Actor 系统
// 关闭流程：
// 1. 标记系统为关闭状态，拒绝新的消息和进程创建
// 2. 遍历所有进程，向每个进程发送关闭任务
// 3. 每个进程通过 mailbox 处理关闭任务，确保在消息处理完成后才退出
// 注意：此方法不会等待所有进程完全退出，进程会在处理完 mailbox 中的消息后自动退出
func (s *System) Shutdown() error {
	// 标记为关闭状态，拒绝新的消息和进程创建
	if !s.shuttingDown.CompareAndSwap(false, true) {
		return nil // 已经在关闭中
	}

	processes := s.GetAllProcesses()
	var lastErr error
	for _, process := range processes {
		// 向进程发送关闭任务，进程会在处理完 mailbox 中的消息后执行退出
		if err := process.Shutdown(); err != nil {
			glog.Error("关闭进程失败", zap.Error(err))
			lastErr = err
		}
	}
	if lastErr != nil {
		return xerror.Wrap(lastErr, "关闭进程时发生错误")
	}
	return nil
}
