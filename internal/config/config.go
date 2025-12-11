package config

import (
	"encoding/json"
	"gas/internal/errs"
	discoveryConfig "gas/pkg/discovery"
	consulConfig "gas/pkg/discovery/provider/consul"
	"gas/pkg/lib"
	"gas/pkg/lib/glog"
	messageQueConfig "gas/pkg/messageQue"
	natsConfig "gas/pkg/messageQue/provider/nats"
	"os"
	"time"
)

// Config 节点配置
type Config struct {
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
		return nil, errs.ErrReadConfigFileFailed(err)
	}
	var config = Default()
	if err = lib.Json.Unmarshal(data, config); err != nil {
		return nil, errs.ErrUnmarshalConfigFailed(err)
	}
	return config, nil
}

// Default 生成默认配置
func Default() *Config {
	return &Config{
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
		Cluster: func() struct {
			Name         string                  `json:"name"`
			Discovery    discoveryConfig.Config  `json:"discovery"`
			MessageQueue messageQueConfig.Config `json:"messageQueue"`
		} {
			// Consul 配置
			consulCfg := consulConfig.Config{
				Address:            "127.0.0.1:8500",
				WatchWaitTime:      5 * time.Second,  // 5秒
				HealthTTL:          10 * time.Second, // 10秒
				DeregisterInterval: 30 * time.Second, // 30秒
			}
			consulCfgJSON, _ := json.Marshal(consulCfg)

			// NATS 配置
			natsCfg := natsConfig.Config{
				Servers:              []string{"nats://127.0.0.1:4222"},
				Name:                 "gas-game-node",
				MaxReconnects:        -1,    // 无限重连
				ReconnectWait:        2000,  // 2秒 = 2000毫秒
				TimeoutMs:            5000,  // 5秒 = 5000毫秒
				PingIntervalMs:       20000, // 20秒 = 20000毫秒
				MaxPingsOut:          2,
				AllowReconnect:       true,
				Username:             "",
				Password:             "",
				Token:                "",
				DisableNoEcho:        false,
				RetryOnFailedConnect: true,
			}
			natsCfgJSON, _ := json.Marshal(natsCfg)

			return struct {
				Name         string                  `json:"name"`
				Discovery    discoveryConfig.Config  `json:"discovery"`
				MessageQueue messageQueConfig.Config `json:"messageQueue"`
			}{
				Name: "cluster.game-node.",
				Discovery: discoveryConfig.Config{
					Type:   "consul",
					Config: consulCfgJSON,
				},
				MessageQueue: messageQueConfig.Config{
					Type:   "nats",
					Config: natsCfgJSON,
				},
			}
		}(),
	}
}
