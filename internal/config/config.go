package config

import (
	"gas/internal/errs"
	discoveryConfig "gas/pkg/discovery"
	"gas/pkg/lib"
	"gas/pkg/lib/glog"
	messageQueConfig "gas/pkg/messageQue"
	"os"
)

type Node struct {
	Id      uint64            `json:"id"`      // 节点ID
	Kind    string            `json:"kind"`    // 节点类型
	Address string            `json:"address"` // 节点地址
	Port    int               `json:"port"`    // 节点端口
	Tags    []string          `json:"tags"`    // 节点标签
	Meta    map[string]string `json:"meta"`    // 节点元数据
}
type Remote struct {
	SubjectPrefix string                   `json:"subject_prefix"`
	Discovery     *discoveryConfig.Config  `json:"discovery"`
	MessageQueue  *messageQueConfig.Config `json:"messageQueue"`
}

// Config 节点配置
type Config struct {
	Node   *Node        `json:"node"`
	Logger *glog.Config `json:"logger"`
	Remote *Remote      `json:"remote"`
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
		Node: &Node{
			Id:      1,
			Kind:    "node1",
			Address: "127.0.0.1",
			Port:    9000,
			Tags:    []string{},
			Meta:    make(map[string]string),
		},
		Logger: &glog.Config{
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
		Remote: &Remote{
			SubjectPrefix: "",
			Discovery: &discoveryConfig.Config{
				Type:   "consul",
				Config: nil,
			},

			MessageQueue: &messageQueConfig.Config{
				Type:   "nats",
				Config: nil,
			},
		},
	}
}
