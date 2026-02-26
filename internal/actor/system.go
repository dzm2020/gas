package actor

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/dzm2020/gas/internal/iface"
	discovery "github.com/dzm2020/gas/pkg/cluster"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib"
	"github.com/dzm2020/gas/pkg/lib/xerror"

	"github.com/duke-git/lancet/v2/maputil"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
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
	autoId       atomic.Uint64
	IdDict       *maputil.ConcurrentMap[uint64, iface.IContext] // ID到进程的映射
	nameDict     *maputil.ConcurrentMap[string, iface.IContext] // 名字到进程ID的映射
	shuttingDown atomic.Bool
	node         iface.INode
}

func NewSystem(node iface.INode) *System {
	return &System{
		node:     node,
		autoId:   atomic.Uint64{},
		IdDict:   maputil.NewConcurrentMap[uint64, iface.IContext](10),
		nameDict: maputil.NewConcurrentMap[string, iface.IContext](10),
	}
}

// ==================== 进程管理 ====================

// Spawn 创建新的 Actor 进程
func (s *System) Spawn(actor iface.IActor, args ...interface{}) *iface.Pid {
	node := s.node
	pid := iface.NewPid(node.GetID(), s.autoId.Add(1))

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
	process := NewProcess(mailBox)
	ctx.process = process

	mailBox.RegisterHandlers(ctx, NewDefaultDispatcher(DefaultDispatcherThroughput))

	s.IdDict.Set(ctx.ID().GetActorId(), ctx)

	// 提交初始化任务，如果失败则记录日志但不影响进程创建
	if err := s.SubmitTask(pid, func(ctx iface.IContext) error {
		return ctx.Actor().OnInit(ctx, args)
	}); err != nil {
		glog.Error("提交Actor初始化任务失败", zap.Any("pid", pid), zap.Error(err))
	}

	return pid
}

// remove 从系统中移除进程
func (s *System) remove(pid *iface.Pid) error {
	s.IdDict.Delete(pid.GetActorId())
	// 尝试取消命名，失败不影响移除操作
	return s.Unname(pid)
}

func (s *System) GetContext(ref any) iface.IContext {
	var ctx iface.IContext
	switch v := ref.(type) {
	case string:
		ctx, _ = s.nameDict.Get(v)
	case uint64:
		ctx, _ = s.IdDict.Get(v)
	case *iface.Pid:
		if v == nil {
			return nil
		}
		if v.GetActorId() > 0 {
			ctx, _ = s.IdDict.Get(v.GetActorId())
		} else if v.GetActorName() != "" {
			ctx, _ = s.nameDict.Get(v.GetActorName())
		}
	}
	return ctx
}

// GetProcess 根据 ref 获取进程，ref 可为 string(名字)、uint64(ActorId)、*Pid
func (s *System) GetProcess(ref any) iface.IProcess {
	ctx := s.GetContext(ref)
	if ctx == nil {
		return nil
	}
	return ctx.Process()
}

// GetAllProcesses 获取系统中所有进程
func (s *System) GetAllProcesses() []iface.IProcess {
	var processes []iface.IProcess
	s.IdDict.Range(func(_ uint64, ctx iface.IContext) bool {
		if ctx != nil {
			processes = append(processes, ctx.Process())
		}
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

	if pid.GetActorName() != "" {
		return ErrNameChangeNotAllowed
	}

	if s.HasName(name) {
		return ErrNameAlreadyRegistered
	}

	ctx, ok := s.IdDict.Get(pid.GetActorId())
	if !ok || ctx == nil {
		return ErrProcessNotFound
	}
	ctx.ID().ActorName = name
	s.nameDict.Set(name, ctx)

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

	info := s.node.Info()
	info.Tags = append(info.Tags, name)

	return cluster.Update(info)
}

// HasName 检查名字是否已注册
func (s *System) HasName(name string) bool {
	_, exists := s.nameDict.Get(name)
	return exists
}

// Unname 注销进程的名字
func (s *System) Unname(pid *iface.Pid) error {
	if pid.GetActorName() == "" {
		return nil
	}

	name := pid.GetActorName()
	s.nameDict.Delete(name)
	ctx, ok := s.IdDict.Get(pid.GetActorId())
	if ok && ctx != nil && ctx.ID() != nil {
		ctx.ID().ActorName = ""
	}

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
	info := s.node.Info()

	info.Tags = slices.DeleteFunc(info.Tags, func(s string) bool {
		return s == name
	})

	return cluster.Update(info)
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
	if err := cluster.Send(message.To.NodeId, message); err != nil {
		return err
	}
	return nil
}

// Call 同步调用 Actor，等待响应
func (s *System) Call(message *iface.ActorMessage) ([]byte, error) {
	timeout := lib.DeadlineToTimeout(message.GetDeadline(), 0)
	if s.isLocalMessage(message) {
		return s.localCall(message)
	}
	cluster := s.node.Cluster()
	if cluster == nil {
		return nil, ErrClusterIsNil
	}
	data, err := cluster.Call(message.To.NodeId, message, timeout)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// localCall 本地同步调用
func (s *System) localCall(message *iface.ActorMessage) (data []byte, err error) {
	timeout := lib.DeadlineToTimeout(message.GetDeadline(), 0)
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
	ctx := s.GetContext(name)
	if ctx != nil {
		return ctx.ID()
	}
	cluster := s.node.Cluster()
	nodeId, err := cluster.Select(name, strategy)
	if err != nil {
		return nil
	}
	return &iface.Pid{
		NodeId:    nodeId,
		ActorName: name,
		ActorId:   0,
	}
}

func (s *System) ShutdownProcess(pid *iface.Pid) error {
	process := s.GetProcess(pid)
	return process.Shutdown()
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
