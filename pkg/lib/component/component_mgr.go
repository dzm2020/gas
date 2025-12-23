package component

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duke-git/lancet/v2/maputil"
)

var (
	ErrComponentCannotBeNil                = errors.New("组件不能为空")
	ErrComponentNameCannotBeEmpty          = errors.New("组件名字不能空")
	ErrCannotRegisterComponentAfterStarted = errors.New("组件启动后无法注册组件")
	ErrComponentAlreadyRegistered          = errors.New("组件已注册")
	ErrManagerAlreadyStarted               = errors.New("管理器已启动")
	ErrManagerStoppedCannotRestart         = errors.New("管理器已停止，无法重启")
	ErrFailedToStartComponent              = errors.New("启动组件失败")
)

type IManager[T any] interface {
	Init(t T) error
	Start(ctx context.Context, t T) error
	Stop(ctx context.Context) error
	ComponentCount() int
	GetComponent(name string) IComponent[T]
	GetComponentNames() []string
	Register(component IComponent[T]) error
}

type Manager[T any] struct {
	components *maputil.ConcurrentMap[string, IComponent[T]]
	order      []string // 保存组件注册顺序
	orderMu    sync.RWMutex
	started    atomic.Bool
	stopped    atomic.Bool
	stopOnce   sync.Once
}

// NewComponentsMgr 创建新的生命周期管理器
func NewComponentsMgr[T any]() *Manager[T] {
	return &Manager[T]{
		components: maputil.NewConcurrentMap[string, IComponent[T]](10),
		order:      make([]string, 0),
	}
}

// IsStarted 检查管理器是否已启动
func (cm *Manager[T]) IsStarted() bool {
	return cm.started.Load()
}

// IsStopped 检查管理器是否已停止
func (cm *Manager[T]) IsStopped() bool {
	return cm.stopped.Load()
}

// ComponentCount 返回已注册的组件数量
func (cm *Manager[T]) ComponentCount() int {
	cm.orderMu.RLock()
	defer cm.orderMu.RUnlock()
	return len(cm.order)
}

func (cm *Manager[T]) GetComponent(name string) IComponent[T] {
	component, _ := cm.components.Get(name)
	return component
}

// GetComponentNames 返回所有已注册组件的名称
func (cm *Manager[T]) GetComponentNames() []string {
	cm.orderMu.RLock()
	defer cm.orderMu.RUnlock()

	names := make([]string, len(cm.order))
	copy(names, cm.order)
	return names
}

// Register 注册组件，按注册顺序启动，按逆序停止
func (cm *Manager[T]) Register(component IComponent[T]) error {
	if cm.started.Load() {
		return ErrCannotRegisterComponentAfterStarted
	}

	if component == nil {
		return ErrComponentCannotBeNil
	}
	if component.Name() == "" {
		return ErrComponentNameCannotBeEmpty
	}

	cm.orderMu.Lock()
	defer cm.orderMu.Unlock()

	// 检查是否已注册同名组件
	if _, exists := cm.components.Get(component.Name()); exists {
		return ErrComponentAlreadyRegistered
	}

	// 注册组件
	cm.components.Set(component.Name(), component)
	cm.order = append(cm.order, component.Name())
	return nil
}

func (cm *Manager[T]) Init(t T) error {
	if cm.started.Load() {
		return ErrManagerAlreadyStarted
	}

	cm.orderMu.RLock()
	order := make([]string, len(cm.order))
	copy(order, cm.order)
	cm.orderMu.RUnlock()

	for _, name := range order {
		component, exists := cm.components.Get(name)
		if !exists {
			continue
		}

		if err := component.Init(t); err != nil {
			return err
		}
	}

	return nil
}

// Start 启动所有已注册的组件
func (cm *Manager[T]) Start(ctx context.Context, t T) error {
	if cm.started.Load() {
		return ErrManagerAlreadyStarted
	}
	if cm.stopped.Load() {
		return ErrManagerStoppedCannotRestart
	}

	cm.orderMu.RLock()
	order := make([]string, len(cm.order))
	copy(order, cm.order)
	cm.orderMu.RUnlock()

	var started []IComponent[T]
	for _, name := range order {
		component, exists := cm.components.Get(name)
		if !exists {
			continue
		}

		if err := component.Start(ctx, t); err != nil {
			return cm.stopComponents(ctx, started)
		}

		started = append(started, component)
	}

	cm.started.Store(true)

	return nil
}

// Stop 停止所有已注册的组件
// 按注册顺序的逆序依次停止，确保依赖关系正确
func (cm *Manager[T]) Stop(ctx context.Context) error {
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
		components := make([]IComponent[T], 0, len(order))
		for i := len(order) - 1; i >= 0; i-- {
			if component, exists := cm.components.Get(order[i]); exists {
				components = append(components, component)
			}
		}

		err = cm.stopComponents(ctx, components)
	})

	return err
}

// stopComponents 停止组件列表（组件列表应该已经是逆序的）
func (cm *Manager[T]) stopComponents(ctx context.Context, components []IComponent[T]) error {
	var lastErr error
	for _, component := range components {
		if component == nil {
			continue
		}
		if err := component.Stop(ctx); err != nil {
			lastErr = err
			continue
		}
	}
	return lastErr
}

// StopWithTimeout 使用超时停止所有组件
func (cm *Manager[T]) StopWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return cm.Stop(ctx)
}
