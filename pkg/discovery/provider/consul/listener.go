package consul

import (
	"gas/pkg/discovery/iface"
	"gas/pkg/utils/glog"
	"reflect"
	"sync"

	"go.uber.org/zap"
)

// serviceListenerManager 服务变化监听器管理器
type serviceListenerManager struct {
	mu        sync.RWMutex
	listeners []iface.ServiceChangeListener
}

// newServiceListenerManager 创建监听器管理器
func newServiceListenerManager() *serviceListenerManager {
	return &serviceListenerManager{
		listeners: make([]iface.ServiceChangeListener, 0),
	}
}

// Add 添加监听器
func (m *serviceListenerManager) Add(listener iface.ServiceChangeListener) {
	if listener == nil {
		return
	}
	m.mu.Lock()
	m.listeners = append(m.listeners, listener)
	m.mu.Unlock()
}

// Remove 移除指定的监听器
func (m *serviceListenerManager) Remove(listener iface.ServiceChangeListener) bool {
	if listener == nil {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	listenerPtr := reflect.ValueOf(listener).Pointer()
	newListeners := make([]iface.ServiceChangeListener, 0, len(m.listeners))
	removed := false

	for _, h := range m.listeners {
		if h != nil && reflect.ValueOf(h).Pointer() != listenerPtr {
			newListeners = append(newListeners, h)
		} else if h != nil {
			removed = true
		}
	}

	if removed {
		m.listeners = newListeners
	}
	return removed
}

// Count 返回当前注册的监听器数量
func (m *serviceListenerManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.listeners)
}

// Notify 通知所有监听器
func (m *serviceListenerManager) Notify(topology *iface.Topology) {
	m.mu.RLock()
	listeners := make([]iface.ServiceChangeListener, len(m.listeners))
	copy(listeners, m.listeners)
	m.mu.RUnlock()

	for _, listener := range listeners {
		if listener != nil {
			go func(l iface.ServiceChangeListener) {
				defer func() {
					if rec := recover(); rec != nil {
						glog.Error("consul watcher listener panic", zap.Any("error", rec))
					}
				}()
				l(topology)
			}(listener)
		}
	}
}
