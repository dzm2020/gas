package component

import (
	"context"
	"gas/internal/errs"
	"gas/internal/iface"
	"gas/pkg/lib/glog"
	"sync"
	"time"

	"go.uber.org/zap"
)

type BaseComponent struct {
}

func (*BaseComponent) Start(ctx context.Context) errs {
	return nil
}
func (*BaseComponent) Stop(ctx context.Context) errs {
	return nil
}

// Component 组件接口，所有需要管理生命周期的组件都应实现此接口
type Component interface {
	// Start 启动组件
	Start(ctx context.Context, node iface.INode) errs
	// Stop 停止组件，ctx 用于控制超时
	Stop(ctx context.Context) errs
	// Name 返回组件名称，用于日志和错误报告
	Name() string
}

// Manager 生命周期管理器
type Manager struct {
	components []Component
	mu         sync.RWMutex
	started    bool
	stopped    bool
	stopOnce   sync.Once
}

// New 创建新的生命周期管理器
func New() *Manager {
	return &Manager{
		components: make([]Component, 0),
	}
}

// Register 注册组件，按注册顺序启动，按逆序停止
func (m *Manager) Register(component Component) errs {
	if component == nil {
		return errs.ErrComponentCannotBeNil()
	}
	if component.Name() == "" {
		return errs.ErrComponentNameCannotBeEmpty()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return errs.ErrCannotRegisterComponentAfterStarted()
	}

	// 检查是否已注册同名组件
	for _, c := range m.components {
		if c.Name() == component.Name() {
			return errs.ErrComponentAlreadyRegistered(component.Name())
		}
	}

	m.components = append(m.components, component)
	glog.Debug("组件: 已注册组件", zap.String("component", component.Name()))
	return nil
}

// Start 启动所有已注册的组件
// 按注册顺序依次启动，如果某个组件启动失败，会停止已启动的组件
func (m *Manager) Start(ctx context.Context, node iface.INode) errs {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return errs.ErrManagerAlreadyStarted()
	}
	if m.stopped {
		m.mu.Unlock()
		return errs.ErrManagerStoppedCannotRestart()
	}
	m.mu.Unlock()

	glog.Info("组件: 正在启动组件", zap.Int("count", len(m.components)))

	var started []Component
	for i, component := range m.components {
		glog.Info("组件: 正在启动组件", zap.String("component", component.Name()), zap.Int("current", i+1), zap.Int("total", len(m.components)))

		if err := component.Start(ctx, node); err != nil {
			glog.Error("组件: 启动组件失败", zap.String("component", component.Name()), zap.Error(err))
			// 停止已启动的组件（逆序）
			m.stopComponents(ctx, started, true)
			return errs.ErrFailedToStartComponent(component.Name(), err)
		}

		started = append(started, component)
		glog.Info("组件: 组件启动成功", zap.String("component", component.Name()))
	}

	m.mu.Lock()
	m.started = true
	m.mu.Unlock()

	glog.Info("组件: 所有组件启动成功", zap.Int("count", len(m.components)))
	return nil
}

// Stop 停止所有已注册的组件
// 按注册顺序的逆序依次停止，确保依赖关系正确
func (m *Manager) Stop(ctx context.Context) errs {
	var err errs
	m.stopOnce.Do(func() {
		m.mu.Lock()
		if !m.started {
			m.mu.Unlock()
			return
		}
		if m.stopped {
			m.mu.Unlock()
			return
		}
		m.stopped = true
		components := make([]Component, len(m.components))
		copy(components, m.components)
		m.mu.Unlock()

		glog.Info("组件: 正在停止组件", zap.Int("count", len(components)))
		err = m.stopComponents(ctx, components, false)
	})

	return err
}

// stopComponents 停止组件列表（逆序）
func (m *Manager) stopComponents(ctx context.Context, components []Component, isRollback bool) errs {
	var lastErr errs
	stopType := "正在停止"
	if isRollback {
		stopType = "正在回滚"
	}

	// 逆序停止
	for i := len(components) - 1; i >= 0; i-- {
		component := components[i]
		if component == nil {
			continue
		}
		glog.Info("组件: 停止组件", zap.String("action", stopType), zap.String("component", component.Name()))

		if err := component.Stop(ctx); err != nil {
			glog.Error("组件: 停止组件失败", zap.String("component", component.Name()), zap.Error(err))
			lastErr = err
			// 继续停止其他组件，不因单个组件失败而中断
		} else {
			glog.Info("组件: 组件停止成功", zap.String("component", component.Name()))
		}
	}

	if !isRollback {
		glog.Info("组件: 所有组件已停止")
	}

	return lastErr
}

// StopWithTimeout 使用超时停止所有组件
func (m *Manager) StopWithTimeout(timeout time.Duration) errs {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return m.Stop(ctx)
}

// IsStarted 检查管理器是否已启动
func (m *Manager) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// IsStopped 检查管理器是否已停止
func (m *Manager) IsStopped() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stopped
}

// ComponentCount 返回已注册的组件数量
func (m *Manager) ComponentCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.components)
}

// GetComponentNames 返回所有已注册组件的名称
func (m *Manager) GetComponentNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, len(m.components))
	for i, c := range m.components {
		names[i] = c.Name()
	}
	return names
}
