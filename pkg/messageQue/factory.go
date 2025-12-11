package messageQue

import (
	"fmt"

	"gas/pkg/messageQue/iface"
	natsProvider "gas/pkg/messageQue/provider/nats"
)

// NewFromConfig 根据配置创建消息队列实例
func NewFromConfig(config Config) (iface.IMessageQue, error) {
	switch config.Type {
	case "nats":
		return natsProvider.New(config.Servers, config.Nats)
	default:
		return nil, fmt.Errorf("unsupported message queue type: %s", config.Type)
	}
}
