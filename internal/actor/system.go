package actor

import (
	"gas/internal/errs"
	"gas/internal/iface"
	"gas/pkg/lib"
	"sync/atomic"
	"time"

	"github.com/duke-git/lancet/v2/maputil"
)

// System 管理本地进程
type System struct {
	uniqId      atomic.Uint64
	processDict *maputil.ConcurrentMap[uint64, iface.IProcess] // ID到进程的映射
	nameDict    *maputil.ConcurrentMap[string, iface.IProcess] // 名字到进程的映射
	//globalNames  *maputil.ConcurrentMap[string, bool]           // 跟踪全局注册的名字
	shuttingDown atomic.Bool
}

// NewSystem 创建新的名字管理器
func NewSystem() *System {
	return &System{
		uniqId:      atomic.Uint64{},
		processDict: maputil.NewConcurrentMap[uint64, iface.IProcess](10),
		nameDict:    maputil.NewConcurrentMap[string, iface.IProcess](10),
		//globalNames: maputil.NewConcurrentMap[string, bool](10),
	}
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

	_ = s.SubmitTask(pid, func(ctx iface.IContext) error {
		return ctx.Actor().OnInit(ctx, args)
	})

	return pid
}

// Add 注册进程
func (s *System) Add(pid *iface.Pid, process iface.IProcess) {
	s.processDict.Set(pid.GetServiceId(), process)
	if pid.GetName() == "" {
		return
	}
	_ = s.RegisterName(pid, process, pid.GetName())
}

func (s *System) RegisterName(pid *iface.Pid, process iface.IProcess, name string) error {
	if len(name) == 0 {
		return errs.ErrNameCannotBeEmpty()
	}

	// 检查1: 如果进程已经有名字
	if pid.Name != "" {
		return errs.ErrNameChangeNotAllowed()
	}

	// 检查2: 名字是否已被其他进程注册
	if s.HasName(name) {
		return errs.ErrNameAlreadyRegistered(name)
	}
	// 注册名字：更新 pid 的名称
	pid.Name = name
	// 在本地注册新名称
	s.nameDict.Set(name, process)
	return nil
}

// registerGlobalName 在集群注册全局名字
func (s *System) registerGlobalName(name string) error {
	iface.GetNode().AddTag(name)
	return iface.GetNode().Cluster().UpdateNode()
}

// HasName 检查名字是否已注册
func (s *System) HasName(name string) bool {
	_, exists := s.nameDict.Get(name)
	return exists
}

// GetProcess 根据 Pid 获取进程
func (s *System) GetProcess(pid *iface.Pid) iface.IProcess {
	if pid == nil {
		return nil
	}
	if pid.GetServiceId() > 0 {
		return s.GetProcessById(pid.GetServiceId())
	}
	if pid.GetName() != "" {
		return s.GetProcessByName(pid.GetName())
	}
	return nil
}

// GetProcessById 根据 ID 获取进程
func (s *System) GetProcessById(id uint64) iface.IProcess {
	process, _ := s.processDict.Get(id)
	return process
}

// GetProcessByName 根据名字获取进程
func (s *System) GetProcessByName(name string) iface.IProcess {
	if name == "" {
		return nil
	}
	process, _ := s.nameDict.Get(name)
	return process
}

func (s *System) Remove(pid *iface.Pid) {
	s.processDict.Delete(pid.GetServiceId())
	s.UnregisterName(pid)
}

// UnregisterName 注销名字
func (s *System) UnregisterName(pid *iface.Pid) {
	if pid.GetName() == "" {
		return
	}
	name := pid.GetName()
	s.nameDict.Delete(name)
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

func (s *System) Call(message *iface.ActorMessage) (data []byte, err error) {
	if err = s.checkShuttingDown(); err != nil {
		return
	}

	process := s.GetProcess(message.GetTo())
	if process == nil {
		err = errs.ErrProcessNotFound
		return
	}

	waiter := lib.NewChanWaiter(message.GetDeadline())
	message.SetResponse(func(bin []byte, e error) {
		data = bin
		err = e
	})

	if err = process.Input(message); err != nil {
		waiter.Done()
		return
	}

	err = waiter.Wait()
	return
}

func (s *System) Send(message *iface.ActorMessage) (err error) {
	if err = s.checkShuttingDown(); err != nil {
		return
	}

	process := s.GetProcess(message.GetTo())
	if process == nil {
		err = errs.ErrProcessNotFound
		return
	}

	return process.Input(message)
}

func (s *System) SubmitTask(to *iface.Pid, task iface.Task) (err error) {
	if err = s.checkShuttingDown(); err != nil {
		return
	}
	process := s.GetProcess(to)
	if process == nil {
		err = errs.ErrProcessNotFound
		return
	}
	msg := iface.NewTaskMessage(task)
	return process.Input(msg)
}

func (s *System) SubmitTaskAndWait(to *iface.Pid, task iface.Task, timeout time.Duration) (err error) {
	if err = s.checkShuttingDown(); err != nil {
		return
	}
	process := s.GetProcess(to)
	if process == nil {
		err = errs.ErrProcessNotFound
		return
	}

	deadline := time.Now().Add(timeout).Unix()
	waiter := lib.NewChanWaiter(deadline)

	syncTask := func(ctx iface.IContext) error {
		if task != nil {
			err = task(ctx)
		}
		waiter.Done()
		return err
	}

	msg := iface.NewTaskMessage(syncTask)

	if err = process.Input(msg); err != nil {
		waiter.Done()
		return
	}

	err = waiter.Wait()

	return
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
