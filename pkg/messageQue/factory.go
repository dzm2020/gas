package messageQue

import (
	"errors"
	"gas/pkg/lib/factory"
	"gas/pkg/messageQue/iface"
)

var (
	fac = factory.New[iface.IMessageQue]()
)

func GetFactoryMgr() *factory.Manager[iface.IMessageQue] {
	return fac
}

// Config 服务发现配置
type Config struct {
	Type   string                 `json:"type"`   // 提供者类型，如 "consul"
	Config map[string]interface{} `json:"config"` // 提供者配置
}

// NewFromConfig 根据配置创建服务发现实例
func NewFromConfig(config Config) (iface.IMessageQue, error) {
	creator, ok := fac.Get(config.Type)
	if !ok {
		return nil, errors.New("unsupported discovery type")
	}
	return creator(config.Config)
}
