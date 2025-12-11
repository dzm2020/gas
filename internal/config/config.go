package config

import (
	"fmt"
	discoveryConfig "gas/pkg/discovery"
	"gas/pkg/lib"
	"gas/pkg/lib/glog"
	messageQueConfig "gas/pkg/messageQue"
	"os"
)

// Config 节点配置
type Config struct {
	// Actor 配置
	Actor struct {
	} `json:"actor"`

	// Node 配置
	Node struct {
		Id      uint64            `json:"id"`      // 节点ID
		Name    string            `json:"name"`    // 节点名称
		Address string            `json:"address"` // 节点地址
		Port    int               `json:"port"`    // 节点端口
		Tags    []string          `json:"tags"`    // 节点标签
		Meta    map[string]string `json:"meta"`    // 节点元数据
	} `json:"game-node"`

	// Glog 配置
	Glog glog.Config `json:"glog"`

	// Cluster 配置
	Cluster struct {
		Name string `json:"name"` // 集群名
		// Discovery 配置
		Discovery discoveryConfig.Config `json:"discovery"`
		// MessageQueue 配置
		MessageQueue messageQueConfig.Config `json:"messageQueue"`
	} `json:"cluster"`
}

func Load(profileFilePath string) (*Config, error) {
	data, err := os.ReadFile(profileFilePath)
	if err != nil {
		return nil, fmt.Errorf("read config file failed: %w", err)
	}
	var config = Default()
	if err = lib.Json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %w", err)
	}
	return config, nil
}

// Default 生成默认配置
func Default() *Config {
	return &Config{
		Actor: struct{}{},
		Node: struct {
			Id      uint64            `json:"id"`
			Name    string            `json:"name"`
			Address string            `json:"address"`
			Port    int               `json:"port"`
			Tags    []string          `json:"tags"`
			Meta    map[string]string `json:"meta"`
		}{
			Id:      1,
			Name:    "game-node-1",
			Address: "127.0.0.1",
			Port:    9000,
			Tags:    []string{},
			Meta:    make(map[string]string),
		},
		Glog: glog.Config{
			Path:         "./logs/app.log",
			Level:        "info",
			PrintConsole: true,
			File: glog.FileConfig{
				MaxSize:    500,
				MaxBackups: 100,
				MaxAge:     30,
				Compress:   false,
				LocalTime:  true,
			},
		},
		Cluster: struct {
			Name         string                  `json:"name"`
			Discovery    discoveryConfig.Config  `json:"discovery"`
			MessageQueue messageQueConfig.Config `json:"messageQueue"`
		}{
			Name: "cluster.game-node.",
			Discovery: discoveryConfig.Config{
				Type:    "consul",
				Address: "127.0.0.1:8500",
				Consul: discoveryConfig.ConsulConfig{
					WatchWaitTimeMs:      5000,  // 5秒
					HealthTTLMs:          10000, // 10秒
					DeregisterIntervalMs: 30000, // 30秒
				},
			},
			MessageQueue: messageQueConfig.Config{
				Type:    "nats",
				Servers: []string{"nats://127.0.0.1:4222"},
				Nats: messageQueConfig.NatsConfig{
					Name:                 "gas-game-node",
					MaxReconnects:        -1,    // 无限重连
					ReconnectWaitMs:      2000,  // 2秒
					TimeoutMs:            5000,  // 5秒
					PingIntervalMs:       20000, // 20秒
					MaxPingsOut:          2,
					AllowReconnect:       true,
					Username:             "",
					Password:             "",
					Token:                "",
					DisableNoEcho:        false,
					RetryOnFailedConnect: true,
				},
			},
		},
	}
}
