package consul

import (
	"time"
)

type Config struct {
	Address            string        `json:"address" mapstruct:"address"`
	WatchWaitTime      time.Duration `json:"watchWaitTime" mapstruct:"watchWaitTime"`
	HealthTTL          time.Duration `json:"healthTTL" mapstruct:"healthTTL"`
	DeregisterInterval time.Duration `json:"deregisterInterval" mapstruct:"deregisterInterval"`
}

func DefaultConfig() *Config {
	return &Config{
		Address:            "127.0.0.1:8500",
		WatchWaitTime:      1 * time.Second,
		HealthTTL:          1 * time.Second,
		DeregisterInterval: 3 * time.Second,
	}
}
