package consul

import (
	"time"
)

type Config struct {
	Address            string
	WatchWaitTime      time.Duration
	HealthTTL          time.Duration
	DeregisterInterval time.Duration
}

func defaultConfig() *Config {
	return &Config{
		Address:            "127.0.0.1:8500",
		WatchWaitTime:      1 * time.Second,
		HealthTTL:          1 * time.Second,
		DeregisterInterval: 3 * time.Second,
	}
}
