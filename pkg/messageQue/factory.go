package messageQue

import (
	"fmt"
	"time"

	"gas/pkg/messageQue/iface"
	natsProvider "gas/pkg/messageQue/provider/nats"
)

// NewFromConfig 根据配置创建消息队列实例
func NewFromConfig(config Config) (iface.IMessageQue, error) {
	switch config.Type {
	case "nats":
		return newNatsFromConfig(config)
	default:
		return nil, fmt.Errorf("unsupported message queue type: %s", config.Type)
	}
}

// newNatsFromConfig 根据配置创建 NATS 消息队列实例
func newNatsFromConfig(config Config) (iface.IMessageQue, error) {
	servers := config.Servers
	if len(servers) == 0 {
		servers = []string{"nats://127.0.0.1:4222"}
	}

	// 构建 nats options
	var natsOpts []natsProvider.Option
	natsConfig := config.Nats

	if natsConfig.Name != "" {
		natsOpts = append(natsOpts, natsProvider.WithName(natsConfig.Name))
	}
	// MaxReconnects: 如果为 0，使用默认值 -1（无限重连）；否则使用配置值
	maxReconnects := natsConfig.MaxReconnects
	if maxReconnects == 0 {
		maxReconnects = -1 // 默认无限重连
	}
	natsOpts = append(natsOpts, natsProvider.WithMaxReconnects(maxReconnects))
	if natsConfig.ReconnectWaitMs > 0 {
		natsOpts = append(natsOpts, natsProvider.WithReconnectWait(time.Duration(natsConfig.ReconnectWaitMs)*time.Millisecond))
	}
	if natsConfig.TimeoutMs > 0 {
		natsOpts = append(natsOpts, natsProvider.WithTimeout(time.Duration(natsConfig.TimeoutMs)*time.Millisecond))
	}
	if natsConfig.PingIntervalMs > 0 {
		natsOpts = append(natsOpts, natsProvider.WithPingInterval(time.Duration(natsConfig.PingIntervalMs)*time.Millisecond))
	}
	if natsConfig.MaxPingsOut > 0 {
		natsOpts = append(natsOpts, natsProvider.WithMaxPingsOut(natsConfig.MaxPingsOut))
	}
	if !natsConfig.AllowReconnect {
		natsOpts = append(natsOpts, natsProvider.WithAllowReconnect(false))
	}
	if natsConfig.Username != "" && natsConfig.Password != "" {
		natsOpts = append(natsOpts, natsProvider.WithUserInfo(natsConfig.Username, natsConfig.Password))
	}
	if natsConfig.Token != "" {
		natsOpts = append(natsOpts, natsProvider.WithToken(natsConfig.Token))
	}

	if natsConfig.RetryOnFailedConnect {
		natsOpts = append(natsOpts, natsProvider.WithRetryOnFailedConnect(true))
	}

	return natsProvider.New(servers, natsOpts...)
}


































