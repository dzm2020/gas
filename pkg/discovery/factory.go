package discovery

import (
	"fmt"
	"time"

	"gas/pkg/discovery/iface"
	consulProvider "gas/pkg/discovery/provider/consul"
)

// NewFromConfig 根据配置创建服务发现实例
func NewFromConfig(config Config) (iface.IDiscovery, error) {
	switch config.Type {
	case "consul":
		return newConsulFromConfig(config)
	default:
		return nil, fmt.Errorf("unsupported discovery type: %s", config.Type)
	}
}

// newConsulFromConfig 根据配置创建 Consul 服务发现实例
func newConsulFromConfig(config Config) (iface.IDiscovery, error) {
	address := config.Address
	if address == "" {
		address = "127.0.0.1:8500"
	}

	// 构建 consul options
	var consulOpts []consulProvider.Option
	consulOpts = append(consulOpts, consulProvider.WithAddress(address))

	consulConfig := config.Consul
	if consulConfig.WatchWaitTimeMs > 0 {
		consulOpts = append(consulOpts, consulProvider.WithWatchWaitTime(time.Duration(consulConfig.WatchWaitTimeMs)*time.Millisecond))
	}
	if consulConfig.HealthTTLMs > 0 {
		consulOpts = append(consulOpts, consulProvider.WithHealthTTL(time.Duration(consulConfig.HealthTTLMs)*time.Millisecond))
	}
	if consulConfig.DeregisterIntervalMs > 0 {
		consulOpts = append(consulOpts, consulProvider.WithDeregisterInterval(time.Duration(consulConfig.DeregisterIntervalMs)*time.Millisecond))
	}

	return consulProvider.New(consulOpts...)
}



















