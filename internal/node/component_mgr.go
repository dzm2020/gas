package node

import (
	"context"
	"gas/internal/errs"
	"gas/internal/iface"
	"gas/pkg/lib/glog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duke-git/lancet/v2/maputil"
	"go.uber.org/zap"
)

// ComponentManager 生命周期管理器
type ComponentManager struct {
	components *maputil.ConcurrentMap[string, iface.IComponent]
	order      []string // 保存组件注册顺序
	orderMu    sync.RWMutex
	started    atomic.Bool
	stopped    atomic.Bool
	stopOnce   sync.Once
}

// NewComponentsMgr 创建新的生命周期管理器
func NewComponentsMgr() *ComponentManager {
	return &ComponentManager{
		components: maputil.NewConcurrentMap[string, iface.IComponent](10),
		order:      make([]string, 0),
	}
}

// IsStarted 检查管理器是否已启动
func (cm *ComponentManager) IsStarted() bool {
	return cm.started.Load()
}

// IsStopped 检查管理器是否已停止
func (cm *ComponentManager) IsStopped() bool {
	return cm.stopped.Load()
}

// ComponentCount 返回已注册的组件数量
func (cm *ComponentManager) ComponentCount() int {
	cm.orderMu.RLock()
	defer cm.orderMu.RUnlock()
	return len(cm.order)
}

func (cm *ComponentManager) GetComponent(name string) iface.IComponent {
	component, _ := cm.components.Get(name)
	return component
}

// GetComponentNames 返回所有已注册组件的名称
func (cm *ComponentManager) GetComponentNames() []string {
	cm.orderMu.RLock()
	defer cm.orderMu.RUnlock()

	names := make([]string, len(cm.order))
	copy(names, cm.order)
	return names
}

// Register 注册组件，按注册顺序启动，按逆序停止
func (cm *ComponentManager) Register(component iface.IComponent) error {
	if cm.started.Load() {
		return errs.ErrCannotRegisterComponentAfterStarted()
	}

	if component == nil {
		return errs.ErrComponentCannotBeNil()
	}
	if component.Name() == "" {
		return errs.ErrComponentNameCannotBeEmpty()
	}

	cm.orderMu.Lock()
	defer cm.orderMu.Unlock()

	// 检查是否已注册同名组件
	if _, exists := cm.components.Get(component.Name()); exists {
		return errs.ErrComponentAlreadyRegistered(component.Name())
	}

	// 注册组件
	cm.components.Set(component.Name(), component)
	cm.order = append(cm.order, component.Name())
	glog.Debug("组件: 已注册组件", zap.String("component", component.Name()))
	return nil
}

// Start 启动所有已注册的组件
func (cm *ComponentManager) Start(ctx context.Context) error {
	if cm.started.Load() {
		return errs.ErrManagerAlreadyStarted()
	}
	if cm.stopped.Load() {
		return errs.ErrManagerStoppedCannotRestart()
	}

	cm.orderMu.RLock()
	order := make([]string, len(cm.order))
	copy(order, cm.order)
	count := len(order)
	cm.orderMu.RUnlock()

	glog.Info("组件: 正在启动组件", zap.Int("count", count))

	var started []iface.IComponent
	for i, name := range order {
		component, exists := cm.components.Get(name)
		if !exists {
			continue
		}

		glog.Info("组件: 正在启动组件", zap.String("component", component.Name()), zap.Int("current", i+1), zap.Int("total", count))

		if err := component.Start(ctx); err != nil {
			glog.Error("组件: 启动组件失败", zap.String("component", component.Name()), zap.Error(err))
			// 停止已启动的组件（逆序）
			_ = cm.stopComponents(ctx, started, true)
			return errs.ErrFailedToStartComponent(component.Name(), err)
		}

		started = append(started, component)
		glog.Info("组件: 组件启动成功", zap.String("component", component.Name()))
	}

	cm.started.Store(true)

	glog.Info("组件: 所有组件启动成功", zap.Int("count", count))
	return nil
}

// Stop 停止所有已注册的组件
// 按注册顺序的逆序依次停止，确保依赖关系正确
func (cm *ComponentManager) Stop(ctx context.Context) error {
	var err error
	cm.stopOnce.Do(func() {
		if !cm.started.Load() {
			return
		}
		if cm.stopped.Load() {
			return
		}
		cm.stopped.Store(true)

		cm.orderMu.RLock()
		order := make([]string, len(cm.order))
		copy(order, cm.order)
		cm.orderMu.RUnlock()

		// 按逆序获取组件
		components := make([]iface.IComponent, 0, len(order))
		for i := len(order) - 1; i >= 0; i-- {
			if component, exists := cm.components.Get(order[i]); exists {
				components = append(components, component)
			}
		}

		glog.Info("组件: 正在停止组件", zap.Int("count", len(components)))
		err = cm.stopComponents(ctx, components, false)
	})

	return err
}

// stopComponents 停止组件列表（组件列表应该已经是逆序的）
func (cm *ComponentManager) stopComponents(ctx context.Context, components []iface.IComponent, isRollback bool) error {
	var lastErr error
	stopType := "正在停止"
	if isRollback {
		stopType = "正在回滚"
	}

	// 按顺序停止（组件列表已经是逆序的）
	for _, component := range components {
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
func (cm *ComponentManager) StopWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return cm.Stop(ctx)
}
