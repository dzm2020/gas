package nats

import (
	"time"

	"github.com/nats-io/nats.go"
)

// Config NATS 消息队列配置
type Config struct {
	Servers              []string      `json:"servers"`              // 消息队列服务器地址列表
	Name                 string        `json:"name"`                 // 客户端名称
	MaxReconnects        int           `json:"maxReconnects"`        // 最大重连次数，-1 表示无限重连
	ReconnectWait        time.Duration `json:"reconnectWait"`        // 重连等待时间
	Timeout              time.Duration `json:"timeout"`              // 连接超时时间
	PingInterval         time.Duration `json:"pingInterval"`         // Ping 间隔
	MaxPingsOut          int           `json:"maxPingsOut"`          // 最大未响应 Ping 数量
	AllowReconnect       bool          `json:"allowReconnect"`       // 是否允许重连
	Username             string        `json:"username"`             // 用户名
	Password             string        `json:"password"`             // 密码
	Token                string        `json:"token"`                // Token 认证
	DisableNoEcho        bool          `json:"disableNoEcho"`        // 禁用 NoEcho
	RetryOnFailedConnect bool          `json:"retryOnFailedConnect"` // 连接失败时重试
	PoolSize             int           `json:"poolSize"`             // 连接池大小，默认 10
}

// defaultConfig 返回默认配置
func defaultConfig() *Config {
	return &Config{
		Servers:              []string{"nats://127.0.0.1:4222"},
		Name:                 "gas-nats-client",
		MaxReconnects:        -1,          // 无限重连
		ReconnectWait:        time.Second, // 2秒 = 2000毫秒
		Timeout:              time.Second, // 5秒 = 5000毫秒
		PingInterval:         time.Second, // 2分钟 = 120000毫秒
		MaxPingsOut:          2,
		AllowReconnect:       true,
		RetryOnFailedConnect: false,
		PoolSize:             10, // 默认连接池大小为 10
	}
}

// toOptions 将 Config 转换为 nats.Option 列表
func toOptions(cfg *Config) []nats.Option {
	var natsOpts []nats.Option

	if cfg.Name != "" {
		natsOpts = append(natsOpts, nats.Name(cfg.Name))
	}

	// MaxReconnects: -1 表示无限重连，>= 0 表示有限重连次数
	// 如果 AllowReconnect 为 false，则设置 MaxReconnects 为 0 来禁用重连
	maxReconnects := cfg.MaxReconnects
	if !cfg.AllowReconnect {
		maxReconnects = 0 // 禁用重连
	}
	natsOpts = append(natsOpts, nats.MaxReconnects(maxReconnects))

	if cfg.ReconnectWait > 0 {
		natsOpts = append(natsOpts, nats.ReconnectWait(cfg.ReconnectWait))
	}

	if cfg.Timeout > 0 {
		natsOpts = append(natsOpts, nats.Timeout(cfg.Timeout))
	}

	if cfg.PingInterval > 0 {
		natsOpts = append(natsOpts, nats.PingInterval(cfg.PingInterval))
	}

	if cfg.MaxPingsOut > 0 {
		natsOpts = append(natsOpts, nats.MaxPingsOutstanding(cfg.MaxPingsOut))
	}

	if cfg.Username != "" && cfg.Password != "" {
		natsOpts = append(natsOpts, nats.UserInfo(cfg.Username, cfg.Password))
	}

	if cfg.Token != "" {
		natsOpts = append(natsOpts, nats.Token(cfg.Token))
	}

	if cfg.RetryOnFailedConnect {
		natsOpts = append(natsOpts, nats.RetryOnFailedConnect(true))
	}

	return natsOpts
}
