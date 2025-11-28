package component

import (
	"context"
	"fmt"
	"gas/pkg/glog"
	"sync"
	"time"
)

// Component 组件接口，所有需要管理生命周期的组件都应实现此接口
type Component interface {
	// Start 启动组件
	Start(ctx context.Context) error
	// Stop 停止组件，ctx 用于控制超时
	Stop(ctx context.Context) error
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
func (m *Manager) Register(component Component) error {
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
	glog.Debugf("component: registered component '%s'", component.Name())
	return nil
}

// Start 启动所有已注册的组件
// 按注册顺序依次启动，如果某个组件启动失败，会停止已启动的组件
func (m *Manager) Start(ctx context.Context) error {
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

	glog.Infof("component: starting %d components", len(m.components))

	var started []Component
	for i, component := range m.components {
		glog.Infof("component: starting component '%s' (%d/%d)", component.Name(), i+1, len(m.components))

		if err := component.Start(ctx); err != nil {
			glog.Errorf("component: failed to start component '%s': %v", component.Name(), err)
			// 停止已启动的组件（逆序）
			m.stopComponents(ctx, started, true)
			return fmt.Errorf("failed to start component '%s': %w", component.Name(), err)
		}

		started = append(started, component)
		glog.Infof("component: component '%s' started successfully", component.Name())
	}

	m.mu.Lock()
	m.started = true
	m.mu.Unlock()

	glog.Infof("component: all %d components started successfully", len(m.components))
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
		components := make([]Component, len(m.components))
		copy(components, m.components)
		m.mu.Unlock()

		glog.Infof("component: stopping %d components", len(components))
		err = m.stopComponents(ctx, components, false)
	})

	return err
}

// stopComponents 停止组件列表（逆序）
func (m *Manager) stopComponents(ctx context.Context, components []Component, isRollback bool) error {
	var lastErr error
	stopType := "stopping"
	if isRollback {
		stopType = "rolling back"
	}

	// 逆序停止
	for i := len(components) - 1; i >= 0; i-- {
		component := components[i]
		glog.Infof("component: %s component '%s'", stopType, component.Name())

		if err := component.Stop(ctx); err != nil {
			glog.Errorf("component: failed to stop component '%s': %v", component.Name(), err)
			lastErr = err
			// 继续停止其他组件，不因单个组件失败而中断
		} else {
			glog.Infof("component: component '%s' stopped successfully", component.Name())
		}
	}

	if !isRollback {
		glog.Infof("component: all components stopped")
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
