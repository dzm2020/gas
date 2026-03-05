package messageQue

import (
	"fmt"

	"github.com/dzm2020/gas/pkg/messageQue/iface"
	"github.com/dzm2020/gas/pkg/messageQue/registry"
)

// Config 服务发现配置
type Config struct {
	Type   string                 `json:"type"`   // 提供者类型，如 "nats"
	Config map[string]interface{} `json:"config"` // 提供者配置
}

// NewFromConfig 根据配置创建消息队列实例
func NewFromConfig(config Config) (iface.IMessageQue, error) {
	creator, ok := registry.GetFactoryMgr().Get(config.Type)
	if !ok {
		return nil, fmt.Errorf("unsupported message queue type:%v", config.Type)
	}
	return creator(config.Config)
}
