package actor

// system.go 实现单节点 Actor 系统（System）：进程创建与注册、名字管理、本地消息与任务派发、优雅关闭。
import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/dzm2020/gas/internal/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib"
	"github.com/dzm2020/gas/pkg/lib/stopper"
	"github.com/dzm2020/gas/pkg/lib/xerror"

	"github.com/duke-git/lancet/v2/maputil"
	"go.uber.org/zap"
)

// 系统级错误
var (
	ErrProcessExiting        = errors.New("进程正在退出")
	ErrProcessNotFound       = errors.New("进程未找到")
	ErrMessageIsNil          = errors.New("消息为空")
	ErrSystemShuttingDown    = errors.New("系统正在关闭")
	ErrNameAlreadyRegistered = errors.New("名字已注册")
)

const (
	DefaultDispatcherThroughput = 1024 // 默认调度器吞吐量
)

// systemWithSessionFactory 可选能力：提供 Session 工厂，用于解耦 actor 与 session 实现。
type systemWithSessionFactory interface {
	SessionFactory() iface.ISessionFactory
}

// spawn 创建并注册新 Actor 进程，投递 OnInit 任务后返回 Pid。
func spawn(s iface.ISystem, actor iface.IActor, args ...interface{}) *iface.Pid {

	pid := iface.NewPid(s.NodeId(), s.NextID())

	ctx := &actorContext{
		process:    nil,
		pid:        pid,
		actor:      actor,
		router:     GetRouterForActor(actor),
		system:     s,
		timeout:    DefaultCallTimeout,
		serializer: s.Serializer(),
	}
	if sf, ok := s.(systemWithSessionFactory); ok {
		ctx.sessionFactory = sf.SessionFactory()
	}

	mailBox := NewMailbox()
	process := NewProcess(mailBox)
	ctx.process = process

	_ = s.Register(ctx)

	mailBox.RegisterHandlers(ctx, NewDefaultDispatcher(DefaultDispatcherThroughput))

	if err := s.SubmitTask(pid, func(ctx iface.IContext) error {
		return ctx.Actor().OnInit(ctx, args)
	}); err != nil {
		glog.Error("提交Actor初始化任务失败", zap.Any("pid", pid), zap.Error(err))
	}

	return pid
}

var _ iface.ISystem = (*System)(nil)

// NewSystem 构造本节点 Actor 系统，node 用于生成 Pid 与获取节点信息。
func NewSystem(selfNodeID uint64, serializer lib.ISerializer) *System {
	return &System{
		selfNodeID: selfNodeID,
		autoId:     atomic.Uint64{},
		serializer: serializer,
		IdDict:     maputil.NewConcurrentMap[uint64, iface.IContext](10),
		nameDict:   maputil.NewConcurrentMap[string, iface.IContext](10),
	}
}

// System 单节点 Actor 系统：维护进程表与名字表，负责本节点内 Spawn/消息/任务/关闭。
type System struct {
	stopper.Stopper
	selfNodeID     uint64
	autoId         atomic.Uint64
	serializer     lib.ISerializer
	IdDict         *maputil.ConcurrentMap[uint64, iface.IContext] // ActorId -> IContext
	nameDict       *maputil.ConcurrentMap[string, iface.IContext] // 名字 -> IContext
	sessionFactory iface.ISessionFactory                           // 可选，用于从 *Session 构造 ISession，由 gate 等上层注入
}

func (s *System) NextID() uint64 {
	return s.autoId.Add(1)
}
func (s *System) NodeId() uint64 {
	return s.selfNodeID
}

func (s *System) Serializer() lib.ISerializer {
	return s.serializer
}

// SetSessionFactory 设置 Session 工厂；传入 nil 表示不使用。由需要 session 能力的上层（如 gate）调用。
func (s *System) SetSessionFactory(f iface.ISessionFactory) {
	s.sessionFactory = f
}

// SessionFactory 实现 systemWithSessionFactory，供 spawn 注入到 actorContext。
func (s *System) SessionFactory() iface.ISessionFactory {
	return s.sessionFactory
}

// Spawn 创建并注册新 Actor 进程，投递 OnInit 任务后返回 Pid。
func (s *System) Spawn(actor iface.IActor, args ...interface{}) *iface.Pid {
	return spawn(s, actor, args...)
}

// Register 将已构建的 Context 注册到系统（写入 IdDict），调用方需保证 Pid 已设置。
func (s *System) Register(ctx iface.IContext) error {
	s.IdDict.Set(ctx.ID().GetActorId(), ctx)
	return nil
}

// Unregister 从系统移除进程（IdDict 删除 + Unname），Unname 失败不影响移除。
func (s *System) Unregister(ctx iface.IContext) error {
	s.IdDict.Delete(ctx.ID().GetActorId())
	return s.Unname(ctx)
}

// getContext 根据 ref 查找 IContext，ref 可为 string | uint64 | *Pid。
func (s *System) getContext(ref any) iface.IContext {
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

// GetProcess 根据 ref 返回进程；ref 支持 string(名字)、uint64(ActorId)、*Pid。
func (s *System) GetProcess(ref any) iface.IProcess {
	ctx := s.getContext(ref)
	if ctx == nil {
		return nil
	}
	return ctx.Process()
}

// GetAllProcesses 返回当前系统中所有已注册进程。
func (s *System) GetAllProcesses() []iface.IProcess {
	var processes []iface.IProcess
	s.IdDict.Range(func(_ uint64, ctx iface.IContext) bool {
		processes = append(processes, ctx.Process())
		return true
	})
	return processes
}

// Named 为进程注册名字，名字已存在则返回 ErrNameAlreadyRegistered。
func (s *System) Named(ctx iface.IContext) error {
	name := ctx.GetName()
	if _, exists := s.nameDict.Get(name); exists {
		return ErrNameAlreadyRegistered
	}
	s.nameDict.Set(ctx.GetName(), ctx)
	return nil
}

// Unname 注销进程当前名字（从 nameDict 删除）。
func (s *System) Unname(ctx iface.IContext) error {
	name := ctx.GetName()
	s.nameDict.Delete(name)
	return nil
}

// Send 向目标进程异步发送消息（仅本节点）。
func (s *System) Send(message *iface.ActorMessage) error {
	return s.sendToProcess(message.To, message)
}

// Call 向目标进程同步发送消息并等待响应，超时由 message.Deadline 决定。
func (s *System) Call(message *iface.ActorMessage) (data []byte, err error) {
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

// SubmitTask 向指定进程投递异步任务。
func (s *System) SubmitTask(to *iface.Pid, task iface.Task) error {
	msg := iface.NewTaskMessage(task)
	return s.sendToProcess(to, msg)
}

// SubmitTaskAndWait 向指定进程投递任务并等待执行完成，超时由 timeout 控制。
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

// sendToProcess 将消息投递到 to 对应进程；关闭中或进程不存在时返回错误。
func (s *System) sendToProcess(to *iface.Pid, msg iface.IMessage) error {
	if s.IsStop() {
		return ErrSystemShuttingDown
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

// ShutdownProcess 向指定进程发送关闭任务，进程会在处理完 mailbox 后退出。
func (s *System) ShutdownProcess(pid *iface.Pid) error {
	process := s.GetProcess(pid)
	return process.Shutdown()
}

// Shutdown 优雅关闭系统：标记关闭状态后向所有进程发送关闭任务；
// 不等待进程完全退出，各进程在处理完 mailbox 后自行退出。
func (s *System) Shutdown() error {
	if !s.Stop() {
		return ErrSystemShuttingDown
	}

	processes := s.GetAllProcesses()
	var lastErr error
	for _, process := range processes {
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
