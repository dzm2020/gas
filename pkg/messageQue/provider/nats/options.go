package nats

import (
	"time"

	"github.com/nats-io/nats.go"
)

// Option 配置选项函数类型
type Option func(*Options)

// Options NATS 客户端配置
type Options struct {
	Name                 string        // 客户端名称
	MaxReconnects        int           // 最大重连次数，-1 表示无限重连
	ReconnectWait        time.Duration // 重连等待时间
	Timeout              time.Duration // 连接超时时间
	PingInterval         time.Duration // Ping 间隔
	MaxPingsOut          int           // 最大未响应 Ping 数量
	AllowReconnect       bool          // 是否允许重连
	Username             string        // 用户名
	Password             string        // 密码
	Token                string        // Token 认证
	RetryOnFailedConnect bool          // 连接失败时重试
}

func defaultOptions() *Options {
	return &Options{
		Name:                 "gas-nats-client",
		MaxReconnects:        -1, // 无限重连
		ReconnectWait:        2 * time.Second,
		Timeout:              5 * time.Second,
		PingInterval:         2 * time.Minute,
		MaxPingsOut:          2,
		AllowReconnect:       true,
		RetryOnFailedConnect: false,
	}
}

func loadOptions(opts ...Option) *Options {
	options := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(options)
		}
	}
	return options
}

// WithName 设置客户端名称
func WithName(name string) Option {
	return func(o *Options) {
		o.Name = name
	}
}

// WithMaxReconnects 设置最大重连次数
func WithMaxReconnects(max int) Option {
	return func(o *Options) {
		o.MaxReconnects = max
	}
}

// WithReconnectWait 设置重连等待时间
func WithReconnectWait(d time.Duration) Option {
	return func(o *Options) {
		o.ReconnectWait = d
	}
}

// WithTimeout 设置连接超时时间
func WithTimeout(d time.Duration) Option {
	return func(o *Options) {
		o.Timeout = d
	}
}

// WithPingInterval 设置 Ping 间隔
func WithPingInterval(d time.Duration) Option {
	return func(o *Options) {
		o.PingInterval = d
	}
}

// WithMaxPingsOut 设置最大未响应 Ping 数量
func WithMaxPingsOut(max int) Option {
	return func(o *Options) {
		o.MaxPingsOut = max
	}
}

// WithAllowReconnect 设置是否允许重连
func WithAllowReconnect(allow bool) Option {
	return func(o *Options) {
		o.AllowReconnect = allow
	}
}

// WithUserInfo 设置用户名和密码
func WithUserInfo(username, password string) Option {
	return func(o *Options) {
		o.Username = username
		o.Password = password
	}
}

// WithToken 设置 Token 认证
func WithToken(token string) Option {
	return func(o *Options) {
		o.Token = token
	}
}

// WithRetryOnFailedConnect 设置连接失败时是否重试
func WithRetryOnFailedConnect(retry bool) Option {
	return func(o *Options) {
		o.RetryOnFailedConnect = retry
	}
}

// buildNatsOptions 将 Options 转换为 nats.Option 列表
func buildNatsOptions(opts *Options) []nats.Option {
	var natsOpts []nats.Option

	if opts.Name != "" {
		natsOpts = append(natsOpts, nats.Name(opts.Name))
	}

	// MaxReconnects: -1 表示无限重连，>= 0 表示有限重连次数
	// 如果 AllowReconnect 为 false，则设置 MaxReconnects 为 0 来禁用重连
	maxReconnects := opts.MaxReconnects
	if !opts.AllowReconnect {
		maxReconnects = 0 // 禁用重连
	}
	natsOpts = append(natsOpts, nats.MaxReconnects(maxReconnects))

	if opts.ReconnectWait > 0 {
		natsOpts = append(natsOpts, nats.ReconnectWait(opts.ReconnectWait))
	}

	if opts.Timeout > 0 {
		natsOpts = append(natsOpts, nats.Timeout(opts.Timeout))
	}

	if opts.PingInterval > 0 {
		natsOpts = append(natsOpts, nats.PingInterval(opts.PingInterval))
	}

	if opts.MaxPingsOut > 0 {
		natsOpts = append(natsOpts, nats.MaxPingsOutstanding(opts.MaxPingsOut))
	}

	if opts.Username != "" && opts.Password != "" {
		natsOpts = append(natsOpts, nats.UserInfo(opts.Username, opts.Password))
	}

	if opts.Token != "" {
		natsOpts = append(natsOpts, nats.Token(opts.Token))
	}

	if opts.RetryOnFailedConnect {
		natsOpts = append(natsOpts, nats.RetryOnFailedConnect(true))
	}

	return natsOpts
}
