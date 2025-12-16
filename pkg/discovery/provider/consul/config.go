package consul

import (
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
