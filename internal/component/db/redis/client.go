package redis

import (
	"context"
	"errors"

	"github.com/redis/go-redis/v9"
)

// IsNil 判断错误是否为 Redis 的 Nil 错误（键不存在）
func IsNil(err error) bool {
	return errors.Is(err, redis.Nil)
}

// Error 判断是否为真正的错误（排除 Nil 错误）
func Error(err error) bool {
	if errors.Is(err, redis.Nil) {
		return false
	}
	return err != nil
}

// Config Redis 配置
type Config struct {
	Address  []string `json:"address" yaml:"address"`     // 地址列表
	Password string   `json:"password" yaml:"password"`   // 密码
	PoolSize int      `json:"pool_size" yaml:"pool_size"` // 连接池大小
}

// Client Redis 客户端
type Client struct {
	redis.UniversalClient
	conf *Config
}

// newClient 创建新的 Redis 客户端
func newClient(config *Config) (*Client, error) {
	client := &Client{
		UniversalClient: redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:    config.Address,
			Password: config.Password,
			PoolSize: config.PoolSize,
		}),
		conf: config,
	}

	// Ping 确保可以正确连接
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return client, nil
}
