package discovery

import (
	"encoding/json"
	"fmt"
	"gas/pkg/discovery/iface"
	"sync"
)

// ProviderFactory 提供者工厂函数，接收 JSON 配置并返回服务发现实例
type ProviderFactory func(configData json.RawMessage) (iface.IDiscovery, error)

var (
	mu        sync.RWMutex
	factories = make(map[string]ProviderFactory)
)

// Config 服务发现配置
type Config struct {
	Type   string          `json:"type"`              // 提供者类型，如 "consul"
	Config json.RawMessage `json:"options,omitempty"` // 提供者配置（JSON）
}

// NewFromConfig 根据配置创建服务发现实例
func NewFromConfig(config Config) (iface.IDiscovery, error) {
	if config.Type == "" {
		return nil, fmt.Errorf("discovery type is required")
	}

	factory, ok := GetFactory(config.Type)
	if !ok {
		return nil, fmt.Errorf("unsupported discovery type: %s (available: %v)", config.Type, ListProviders())
	}
	return factory(config.Config)
}

// Register 注册服务发现提供者
func Register(name string, factory ProviderFactory) error {
	if name == "" || factory == nil {
		return fmt.Errorf("provider name and factory cannot be empty")
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := factories[name]; ok {
		return fmt.Errorf("provider %s is already registered", name)
	}
	factories[name] = factory
	return nil
}

// Unregister 注销服务发现提供者
func Unregister(name string) {
	mu.Lock()
	defer mu.Unlock()
	delete(factories, name)
}

// GetFactory 获取指定名称的工厂函数
func GetFactory(name string) (ProviderFactory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	factory, ok := factories[name]
	return factory, ok
}

// ListProviders 列出所有已注册的提供者名称
func ListProviders() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(factories))
	for name := range factories {
		names = append(names, name)
	}
	return names
}
