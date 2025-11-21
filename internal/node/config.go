package node

import (
	"fmt"
	discoveryConfig "gas/pkg/discovery"
	messageQueConfig "gas/pkg/messageQue"
	"gas/pkg/utils/serializer"
	"os"
)

// loadConfig 加载配置文件
func loadConfig(profileFilePath string) (*Config, error) {
	data, err := os.ReadFile(profileFilePath)
	if err != nil {
		return nil, fmt.Errorf("read config file failed: %w", err)
	}

	var config Config
	if err := serializer.Json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %w", err)
	}

	// 设置默认值
	if config.Cluster.NodeSubjectPrefix == "" {
		config.Cluster.NodeSubjectPrefix = "cluster.node."
	}

	return &config, nil
}

// Config 节点配置
type Config struct {
	// Actor 配置
	Actor struct {
	} `json:"actor"`

	// Discovery 配置
	Discovery discoveryConfig.Config `json:"discovery"`

	// MessageQueue 配置
	MessageQueue messageQueConfig.Config `json:"messageQueue"`

	// Node 配置
	Node struct {
		Id      uint64            `json:"id"`      // 节点ID
		Name    string            `json:"name"`    // 节点名称
		Address string            `json:"address"` // 节点地址
		Port    int               `json:"port"`    // 节点端口
		Tags    []string          `json:"tags"`    // 节点标签
		Meta    map[string]string `json:"meta"`    // 节点元数据
	} `json:"node"`

	// Cluster 配置
	Cluster struct {
		NodeSubjectPrefix string `json:"nodeSubjectPrefix"` // 节点主题前缀，默认为 "cluster.node."
	} `json:"cluster"`
}
