package messageQue

import (
	"encoding/json"
	"fmt"
	"sync"

	"gas/pkg/messageQue/iface"
)

// ProviderFactory 消息队列提供者工厂函数
// configData 提供者的配置数据（JSON 格式）
type ProviderFactory func(configData json.RawMessage) (iface.IMessageQue, error)

var (
	mu        sync.RWMutex
	factories = make(map[string]ProviderFactory)
)

// Config 消息队列配置
type Config struct {
	Type   string          `json:"type"`              // 消息队列类型，如 "nats"
	Config json.RawMessage `json:"options,omitempty"` // 提供者特定配置（JSON 格式）
}

// NewFromConfig 根据配置创建消息队列实例
// 支持通过注册器动态创建不同类型的消息队列
func NewFromConfig(config Config) (iface.IMessageQue, error) {
	if config.Type == "" {
		return nil, fmt.Errorf("message queue type is required")
	}
	// 从注册表中获取工厂函数
	factory, exists := GetFactory(config.Type)
	if !exists {
		return nil, fmt.Errorf("unsupported message queue type: %s (available: %v)", config.Type, ListProviders())
	}
	return factory(config.Config)
}

// Register 注册消息队列提供者
// name: 提供者名称（如 "nats", "kafka", "rabbitmq" 等）
// factory: 工厂函数，用于创建该类型的消息队列实例
func Register(name string, factory ProviderFactory) error {
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if factory == nil {
		return fmt.Errorf("provider factory cannot be nil")
	}

	mu.Lock()
	defer mu.Unlock()

	if _, exists := factories[name]; exists {
		return fmt.Errorf("provider %s is already registered", name)
	}

	factories[name] = factory
	return nil
}

// Unregister 注销消息队列提供者
func Unregister(name string) {
	mu.Lock()
	defer mu.Unlock()
	delete(factories, name)
}

// GetFactory 获取指定名称的工厂函数
func GetFactory(name string) (ProviderFactory, bool) {
	mu.RLock()
	defer mu.RUnlock()
	factory, exists := factories[name]
	return factory, exists
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
