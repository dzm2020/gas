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
	node        iface.INode                                    // 节点实例，用于全局注册
}

// NewNameManager 创建新的名字管理器
func NewNameManager() *Manager {
	return &Manager{
		processDict: maputil.NewConcurrentMap[uint64, iface.IProcess](10),
		nameDict:    maputil.NewConcurrentMap[string, iface.IProcess](10),
		globalNames: maputil.NewConcurrentMap[string, bool](10),
	}
}

// SetNode 设置节点实例
func (m *Manager) SetNode(node iface.INode) {
	m.node = node
}

// Add 注册进程
func (m *Manager) Add(pid *iface.Pid, process iface.IProcess) {
	m.processDict.Set(pid.GetServiceId(), process)
	if pid.GetName() == "" {
		return
	}
	_ = m.RegisterName(pid, process, pid.GetName(), false)
}

func (m *Manager) RegisterName(pid *iface.Pid, process iface.IProcess, name string, isGlobal bool) error {
	if len(name) == 0 {
		return errs.ErrNameCannotBeEmpty()
	}

	// 检查1: 如果进程已经有名字
	if pid.Name != "" {
		return errs.ErrNameChangeNotAllowed()
	}

	// 检查2: 名字是否已被其他进程注册
	if m.HasName(name) {
		return errs.ErrNameAlreadyRegistered(name)
	}

	// 如果是全局注册，还需要在远程注册
	if isGlobal {
		if err := m.registerGlobalName(name); err != nil {
			return errs.ErrRemoteRegistryNameFailed(err)
		}
		// 记录全局注册的名字
		m.globalNames.Set(name, true)
	}

	// 注册名字：更新 pid 的名称
	pid.Name = name

	// 在本地注册新名称
	m.nameDict.Set(name, process)

	return nil
}

// registerGlobalName 在远程注册全局名字
func (m *Manager) registerGlobalName(name string) error {
	info := m.node.Info()
	info.Tags = append(info.Tags, name)
	return m.node.GetRemote().UpdateNode()
}

// HasName 检查名字是否已注册
func (m *Manager) HasName(name string) bool {
	_, exists := m.nameDict.Get(name)
	return exists
}

// GetProcess 根据 Pid 获取进程
func (m *Manager) GetProcess(pid *iface.Pid) iface.IProcess {
	if pid == nil {
		return nil
	}
	if pid.GetServiceId() > 0 {
		return m.GetProcessById(pid.GetServiceId())
	}
	if pid.GetName() != "" {
		return m.GetProcessByName(pid.GetName())
	}
	return nil
}

// GetProcessById 根据 ID 获取进程
func (m *Manager) GetProcessById(id uint64) iface.IProcess {
	process, _ := m.processDict.Get(id)
	return process
}

// GetProcessByName 根据名字获取进程
func (m *Manager) GetProcessByName(name string) iface.IProcess {
	if name == "" {
		return nil
	}
	process, _ := m.nameDict.Get(name)
	return process
}

func (m *Manager) Remove(pid *iface.Pid) {
	m.processDict.Delete(pid.GetServiceId())
	m.UnregisterName(pid)
}

// UnregisterName 注销名字
func (m *Manager) UnregisterName(pid *iface.Pid) {
	if pid.GetName() == "" {
		return
	}
	name := pid.GetName()
	m.nameDict.Delete(name)
	// 如果是全局注册的名字，需要从远程注销
	if _, isGlobal := m.globalNames.Get(name); isGlobal {
		m.unregisterGlobalName(name)
		m.globalNames.Delete(name)
	}
}

// unregisterGlobalName 从远程注销全局名字
func (m *Manager) unregisterGlobalName(name string) {
	if m.node == nil {
		return
	}
	remote := m.node.GetRemote()
	info := m.node.Info()
	slices.DeleteFunc(info.Tags, func(s string) bool {
		return s == name
	})
	_ = remote.UpdateNode()
}

// GetAllProcesses 获取所有进程
func (m *Manager) GetAllProcesses() []iface.IProcess {
	var processes []iface.IProcess
	m.processDict.Range(func(_ uint64, value iface.IProcess) bool {
		processes = append(processes, value)
		return true
	})
	return processes
}
