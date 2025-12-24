package consul

import (
	"fmt"
	"time"
)

type Config struct {
	Address            string        `json:"address"`
	WatchWaitTime      time.Duration `json:"watchWaitTime"`
	HealthTTL          time.Duration `json:"healthTTL"`
	DeregisterInterval time.Duration `json:"deregisterInterval"`
}

func defaultConfig() *Config {
	return &Config{
		Address:            "127.0.0.1:8500",
		WatchWaitTime:      10 * time.Second,
		HealthTTL:          1 * time.Second,
		DeregisterInterval: 3 * time.Second,
	}
}

// Validate 验证配置是否有效
func (c *Config) Validate() error {
	if c.Address == "" {
		return fmt.Errorf("consul address cannot be empty")
	}
	if c.HealthTTL <= 0 {
		return fmt.Errorf("consul health TTL must be greater than 0")
	}
	if c.DeregisterInterval <= 0 {
		return fmt.Errorf("consul deregister interval must be greater than 0")
	}
	if c.WatchWaitTime <= 0 {
		return fmt.Errorf("consul watch wait time must be greater than 0")
	}
	return nil
}
