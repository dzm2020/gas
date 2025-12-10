// Package actor 提供 Actor 模型实现，包括进程管理、消息路由、定时器等核心功能
package actor

import (
	"gas/internal/iface"

	"github.com/duke-git/lancet/v2/maputil"
)

// NameManager 名字管理器，负责管理进程名字的注册和注销
type NameManager struct {
	nameDict    *maputil.ConcurrentMap[string, iface.IProcess] // 名字到进程的映射
	globalNames *maputil.ConcurrentMap[string, bool]           // 跟踪全局注册的名字
	node        iface.INode                                    // 节点实例，用于全局注册
}

// NewNameManager 创建新的名字管理器
func NewNameManager() *NameManager {
	return &NameManager{
		nameDict:    maputil.NewConcurrentMap[string, iface.IProcess](10),
		globalNames: maputil.NewConcurrentMap[string, bool](10),
	}
}

// SetNode 设置节点实例
func (nm *NameManager) SetNode(node iface.INode) {
	nm.node = node
}

// Register 注册名字
// pid: 进程ID
// process: 进程实例
// name: 要注册的名字
// isGlobal: 是否为全局注册
// 约束：
//   - 一个名字只能注册一次
//   - 每个进程只能注册一次名字
//   - 不允许更名
func (nm *NameManager) Register(pid *iface.Pid, process iface.IProcess, name string, isGlobal bool) error {
	if len(name) == 0 {
		return ErrNameCannotBeEmpty()
	}

	oldName := pid.Name

	// 检查1: 如果进程已经有名字
	if oldName != "" {
		return ErrNameChangeNotAllowed()
	}

	// 检查2: 名字是否已被其他进程注册
	if nm.HasName(name) {
		return ErrNameAlreadyRegistered(name)
	}

	// 如果是全局注册，还需要在远程注册
	if isGlobal {
		if err := nm.registerGlobalName(name); err != nil {
			return ErrRemoteRegistryNameFailed(err)
		}
		// 记录全局注册的名字
		nm.globalNames.Set(name, true)
	}

	// 注册名字：更新 pid 的名称
	pid.Name = name

	// 在本地注册新名称
	nm.nameDict.Set(name, process)

	return nil
}

// Unregister 注销名字
func (nm *NameManager) Unregister(pid *iface.Pid) {
	if pid.GetName() == "" {
		return
	}

	name := pid.GetName()

	nm.nameDict.Delete(name)

	// 如果是全局注册的名字，需要从远程注销
	if _, isGlobal := nm.globalNames.Get(name); isGlobal {
		nm.unregisterGlobalName(name)
		nm.globalNames.Delete(name)
	}
}

// GetProcess 根据名字获取进程
func (nm *NameManager) GetProcess(name string) iface.IProcess {
	if name == "" {
		return nil
	}
	process, _ := nm.nameDict.Get(name)
	return process
}

// HasName 检查名字是否已注册
func (nm *NameManager) HasName(name string) bool {
	_, exists := nm.nameDict.Get(name)
	return exists
}

// Set 直接设置名字到进程的映射（用于进程注册时，仅当进程创建时已有名字）
// 注意：此方法不进行约束检查，因为这是进程创建时的初始名字设置
func (nm *NameManager) Set(name string, process iface.IProcess) {
	if name == "" || process == nil {
		return
	}
	// 获取进程ID
	pid := process.Context().ID()
	if pid == nil {
		return
	}
	// 检查名字是否已被注册
	if nm.HasName(name) {
		return
	}
	nm.nameDict.Set(name, process)
}

// registerGlobalName 在远程注册全局名字
func (nm *NameManager) registerGlobalName(name string) error {
	if nm.node == nil {
		return ErrNodeNotInitialized()
	}
	remote := nm.node.GetRemote()
	if remote == nil {
		return ErrRemoteNotInitialized()
	}
	return remote.RegistryName(name)
}

// unregisterGlobalName 从远程注销全局名字
func (nm *NameManager) unregisterGlobalName(name string) {
	if nm.node == nil {
		return
	}
	remote := nm.node.GetRemote()
	if remote == nil {
		return
	}
	_ = remote.UnregisterName(name)
}
