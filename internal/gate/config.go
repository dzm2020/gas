package gate

import (
	"time"

	"gas/pkg/network"
)

// defaultConfig 生成默认网关配置
func defaultConfig() *Config {
	return &Config{
		Address:      "tcp://127.0.0.1:9000",
		KeepAlive:    5, // 5秒
		SendChanSize: 1024,
		ReadBufSize:  4096,
		MaxConn:      10000,
	}
}

type Config struct {
	// Address 监听地址，格式: "tcp://127.0.0.1:9002" 或 "udp://127.0.0.1:9002"
	Address string `json:"address" yaml:"address"`
	// KeepAlive 连接超时时间（秒），0表示不检测超时
	KeepAlive int `json:"keepAlive,omitempty" yaml:"keepAlive,omitempty"`
	// SendChanSize 发送队列缓冲大小
	SendChanSize int `json:"sendChanSize,omitempty" yaml:"sendChanSize,omitempty"`
	// ReadBufSize 读缓冲区大小
	ReadBufSize int `json:"readBufSize,omitempty" yaml:"readBufSize,omitempty"`
	// MaxConn 最大连接数
	MaxConn int `json:"maxConn,omitempty" yaml:"maxConn,omitempty"`
}

func ToOptions(c *Config) []network.Option {
	var options []network.Option
	if c.KeepAlive > 0 {
		options = append(options, network.WithKeepAlive(time.Duration(c.KeepAlive)*time.Second))
	}
	if c.SendChanSize > 0 {
		options = append(options, network.WithSendChanSize(c.SendChanSize))
	}
	if c.ReadBufSize > 0 {
		options = append(options, network.WithReadBufSize(c.ReadBufSize))
	}
	return options
}
