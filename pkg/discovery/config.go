package discovery

// Config 服务发现配置
type Config struct {
	Type    string `json:"type"`    // 服务发现类型，如 "consul"
	Address string `json:"address"` // 服务发现地址，如 "127.0.0.1:8500"
	Consul  ConsulConfig `json:"consul"`
}

// ConsulConfig Consul 服务发现配置
type ConsulConfig struct {
	WatchWaitTimeMs      int `json:"watchWaitTimeMs"`      // 监听等待时间（毫秒）
	HealthTTLMs          int `json:"healthTTLMs"`          // 健康检查 TTL（毫秒）
	DeregisterIntervalMs int `json:"deregisterIntervalMs"` // 注销间隔（毫秒）
}



















