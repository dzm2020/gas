package actor

import (
	"gas/internal/errs"
	"gas/internal/iface"

	"github.com/duke-git/lancet/v2/maputil"
	"golang.org/x/exp/slices"
)

// Manager 管理进程
type Manager struct {
	processDict *maputil.ConcurrentMap[uint64, iface.IProcess] // ID到进程的映射
	nameDict    *maputil.ConcurrentMap[string, iface.IProcess] // 名字到进程的映射
	globalNames *maputil.ConcurrentMap[string, bool]           // 跟踪全局注册的名字
}

// NewNameManager 创建新的名字管理器
func NewNameManager() *Manager {
	return &Manager{
		processDict: maputil.NewConcurrentMap[uint64, iface.IProcess](10),
		nameDict:    maputil.NewConcurrentMap[string, iface.IProcess](10),
		globalNames: maputil.NewConcurrentMap[string, bool](10),
	}
}

// Add 注册进程
func (mgr *Manager) Add(pid *iface.Pid, process iface.IProcess) {
	mgr.processDict.Set(pid.GetServiceId(), process)
	if pid.GetName() == "" {
		return
	}
	_ = mgr.RegisterName(pid, process, pid.GetName(), false)
}

func (mgr *Manager) RegisterName(pid *iface.Pid, process iface.IProcess, name string, isGlobal bool) error {
	if len(name) == 0 {
		return errs.ErrNameCannotBeEmpty()
	}

	// 检查1: 如果进程已经有名字
	if pid.Name != "" {
		return errs.ErrNameChangeNotAllowed()
	}

	// 检查2: 名字是否已被其他进程注册
	if mgr.HasName(name) {
		return errs.ErrNameAlreadyRegistered(name)
	}

	// 如果是全局注册，还需要在远程注册
	if isGlobal {
		if err := mgr.registerGlobalName(name); err != nil {
			return errs.ErrRemoteRegistryNameFailed(err)
		}
		// 记录全局注册的名字
		mgr.globalNames.Set(name, true)
	}

	// 注册名字：更新 pid 的名称
	pid.Name = name

	// 在本地注册新名称
	mgr.nameDict.Set(name, process)

	return nil
}

// registerGlobalName 在远程注册全局名字
func (mgr *Manager) registerGlobalName(name string) error {
	info := iface.GetNode().Info()
	info.Tags = append(info.Tags, name)
	return iface.GetNode().Remote().UpdateNode()
}

// HasName 检查名字是否已注册
func (mgr *Manager) HasName(name string) bool {
	_, exists := mgr.nameDict.Get(name)
	return exists
}

// GetProcess 根据 Pid 获取进程
func (mgr *Manager) GetProcess(pid *iface.Pid) iface.IProcess {
	if pid == nil {
		return nil
	}
	if pid.GetServiceId() > 0 {
		return mgr.GetProcessById(pid.GetServiceId())
	}
	if pid.GetName() != "" {
		return mgr.GetProcessByName(pid.GetName())
	}
	return nil
}

// GetProcessById 根据 ID 获取进程
func (mgr *Manager) GetProcessById(id uint64) iface.IProcess {
	process, _ := mgr.processDict.Get(id)
	return process
}

// GetProcessByName 根据名字获取进程
func (mgr *Manager) GetProcessByName(name string) iface.IProcess {
	if name == "" {
		return nil
	}
	process, _ := mgr.nameDict.Get(name)
	return process
}

func (mgr *Manager) Remove(pid *iface.Pid) {
	mgr.processDict.Delete(pid.GetServiceId())
	mgr.UnregisterName(pid)
}

// UnregisterName 注销名字
func (mgr *Manager) UnregisterName(pid *iface.Pid) {
	if pid.GetName() == "" {
		return
	}
	name := pid.GetName()
	mgr.nameDict.Delete(name)
	// 如果是全局注册的名字，需要从远程注销
	if _, isGlobal := mgr.globalNames.Get(name); isGlobal {
		mgr.unregisterGlobalName(name)
		mgr.globalNames.Delete(name)
	}
}

// unregisterGlobalName 从远程注销全局名字
func (mgr *Manager) unregisterGlobalName(name string) {
	remote := iface.GetNode().Remote()
	info := iface.GetNode().Info()
	slices.DeleteFunc(info.Tags, func(s string) bool {
		return s == name
	})
	_ = remote.UpdateNode()
}

// GetAllProcesses 获取所有进程
func (mgr *Manager) GetAllProcesses() []iface.IProcess {
	var processes []iface.IProcess
	mgr.processDict.Range(func(_ uint64, value iface.IProcess) bool {
		processes = append(processes, value)
		return true
	})
	return processes
}
