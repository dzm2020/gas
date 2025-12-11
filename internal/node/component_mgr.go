package node

import (
	"context"
	"fmt"
	"gas/internal/iface"
	"gas/pkg/lib/glog"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 生命周期管理器
type Manager struct {
	components []iface.IComponent
	mu         sync.RWMutex
	started    bool
	stopped    bool
	stopOnce   sync.Once
}

// NewComponentsMgr 创建新的生命周期管理器
func NewComponentsMgr() *Manager {
	return &Manager{
		components: make([]iface.IComponent, 0),
	}
}

// Register 注册组件，按注册顺序启动，按逆序停止
func (m *Manager) Register(component iface.IComponent) error {
	if component == nil {
		return fmt.Errorf("component cannot be nil")
	}
	if component.Name() == "" {
		return fmt.Errorf("component name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("cannot register component after manager has started")
	}

	// 检查是否已注册同名组件
	for _, c := range m.components {
		if c.Name() == component.Name() {
			return fmt.Errorf("component with name '%s' already registered", component.Name())
		}
	}

	m.components = append(m.components, component)
	glog.Debug("组件: 已注册组件", zap.String("component", component.Name()))
	return nil
}

// Start 启动所有已注册的组件
// 按注册顺序依次启动，如果某个组件启动失败，会停止已启动的组件
func (m *Manager) Start(ctx context.Context, node iface.INode) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return fmt.Errorf("manager has already been started")
	}
	if m.stopped {
		m.mu.Unlock()
		return fmt.Errorf("manager has been stopped and cannot be restarted")
	}
	m.mu.Unlock()

	glog.Info("组件: 正在启动组件", zap.Int("count", len(m.components)))

	var started []iface.IComponent
	for i, component := range m.components {
		glog.Info("组件: 正在启动组件", zap.String("component", component.Name()), zap.Int("current", i+1), zap.Int("total", len(m.components)))

		if err := component.Start(ctx, node); err != nil {
			glog.Error("组件: 启动组件失败", zap.String("component", component.Name()), zap.Error(err))
			// 停止已启动的组件（逆序）
			m.stopComponents(ctx, started, true)
			return fmt.Errorf("failed to start component '%s': %w", component.Name(), err)
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
func (m *Manager) Stop(ctx context.Context) error {
	var err error
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
		components := make([]iface.IComponent, len(m.components))
		copy(components, m.components)
		m.mu.Unlock()

		glog.Info("组件: 正在停止组件", zap.Int("count", len(components)))
		err = m.stopComponents(ctx, components, false)
	})

	return err
}

// stopComponents 停止组件列表（逆序）
func (m *Manager) stopComponents(ctx context.Context, components []iface.IComponent, isRollback bool) error {
	var lastErr error
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
func (m *Manager) StopWithTimeout(timeout time.Duration) error {
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
