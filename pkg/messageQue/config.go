package messageQue

import "gas/pkg/messageQue/provider/nats"

// Config 消息队列配置
type Config struct {
	Type    string       `json:"type"`    // 消息队列类型，如 "nats"
	Servers []string     `json:"servers"` // 消息队列服务器地址列表
	Nats    *nats.Config `json:"nats"`
}
