package config

import (
	discoveryConfig "gas/pkg/discovery"
	"gas/pkg/glog"
	"gas/pkg/lib"
	messageQueConfig "gas/pkg/messageQue"
)

type (

	// Config 节点配置
	Config struct {
		Node    *Node        `json:"node"`
		Logger  *glog.Config `json:"logger"`
		Cluster *Cluster     `json:"cluster"`
		Gate    *GateConfig  `json:"gate,omitempty"`
	}

	Node struct {
		Id      uint64            `json:"id"`      // 节点ID
		Kind    string            `json:"kind"`    // 节点类型
		Address string            `json:"address"` // 节点地址
		Port    int               `json:"port"`    // 节点端口
		Tags    []string          `json:"tags"`    // 节点标签
		Meta    map[string]string `json:"meta"`    // 节点元数据
	}
	Cluster struct {
		Name         string                   `json:"name"`
		Discovery    *discoveryConfig.Config  `json:"discovery"`
		MessageQueue *messageQueConfig.Config `json:"messageQueue"`
	}

	GateConfig struct {
		// Address 监听地址，格式: "tcp://127.0.0.1:9002" 或 "udp://127.0.0.1:9002"
		Address string `json:"address"`
		// KeepAlive 连接超时时间（秒），0表示不检测超时
		KeepAlive int `json:"keepAlive,omitempty"`
		// SendChanSize 发送队列缓冲大小
		SendChanSize int `json:"sendChanSize,omitempty"`
		// ReadBufSize 读缓冲区大小
		ReadBufSize int `json:"readBufSize,omitempty"`
		// MaxConn 最大连接数
		MaxConn int `json:"maxConn,omitempty"`
	}
)

func Load(path string) (config *Config, err error) {
	config = Default()
	if err = lib.LoadJsonFile(path, config); err != nil {
		return
	}
	return config, nil
}

// Default 生成默认配置
func Default() *Config {
	return &Config{
		Node:    defaultNode(),
		Logger:  defaultLogger(),
		Cluster: defaultCluster(),
		Gate:    defaultGateConfig(),
	}
}

func defaultLogger() *glog.Config {
	return &glog.Config{
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
	}
}

func defaultNode() *Node {
	return &Node{
		Id:      1,
		Kind:    "node1",
		Address: "127.0.0.1",
		Port:    9000,
		Tags:    []string{},
		Meta:    make(map[string]string),
	}
}

func defaultCluster() *Cluster {
	return &Cluster{
		Name: "",
		Discovery: &discoveryConfig.Config{
			Type:   "consul",
			Config: nil,
		},

		MessageQueue: &messageQueConfig.Config{
			Type:   "nats",
			Config: nil,
		},
	}
}

// defaultGateConfig 生成默认网关配置
func defaultGateConfig() *GateConfig {
	return &GateConfig{
		Address:      "tcp://127.0.0.1:9000",
		KeepAlive:    5, // 5秒
		SendChanSize: 1024,
		ReadBufSize:  4096,
		MaxConn:      10000,
	}
}
