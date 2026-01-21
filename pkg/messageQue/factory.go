package messageQue

import (
	"errors"
	"github.com/dzm2020/gas/pkg/lib/factory"
	"github.com/dzm2020/gas/pkg/messageQue/iface"
)

var (
	factoryMgr = factory.New[iface.IMessageQue]()
)

func GetFactoryMgr() *factory.Manager[iface.IMessageQue] {
	return factoryMgr
}

// Config 服务发现配置
type Config struct {
	Type   string                 `json:"type"`   // 提供者类型，如 "nats"
	Config map[string]interface{} `json:"config"` // 提供者配置
}

// NewFromConfig 根据配置创建服务发现实例
func NewFromConfig(config Config) (iface.IMessageQue, error) {
	creator, ok := factoryMgr.Get(config.Type)
	if !ok {
		return nil, errors.New("unsupported discovery type")
	}
	return creator(config.Config)
}
