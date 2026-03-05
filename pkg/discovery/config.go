package discovery

import (
	"fmt"

	"github.com/dzm2020/gas/pkg/discovery/iface"

	"github.com/dzm2020/gas/pkg/discovery/registry"
)

// Config 服务发现配置
type Config struct {
	Type   string                 `json:"type"`   // 提供者类型，如 "consul"
	Config map[string]interface{} `json:"config"` // 提供者配置
}

// NewFromConfig 根据配置创建服务发现实例
func NewFromConfig(config Config) (iface.IDiscovery, error) {
	creator, ok := registry.GetFactoryMgr().Get(config.Type)
	if !ok {
		return nil, fmt.Errorf("unsupported discovery type:%v", config.Type)
	}
	return creator(config.Config)
}
