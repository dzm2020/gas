package messageQue

// Config 消息队列配置
type Config struct {
	Type    string   `json:"type"`    // 消息队列类型，如 "nats"
	Servers []string `json:"servers"` // 消息队列服务器地址列表
	Nats    NatsConfig `json:"nats"`
}

// NatsConfig NATS 消息队列配置
type NatsConfig struct {
	Name                 string `json:"name"`                 // 客户端名称
	MaxReconnects        int    `json:"maxReconnects"`        // 最大重连次数，-1 表示无限重连
	ReconnectWaitMs      int    `json:"reconnectWaitMs"`      // 重连等待时间（毫秒）
	TimeoutMs            int    `json:"timeoutMs"`            // 连接超时时间（毫秒）
	PingIntervalMs       int    `json:"pingIntervalMs"`       // Ping 间隔（毫秒）
	MaxPingsOut          int    `json:"maxPingsOut"`          // 最大未响应 Ping 数量
	AllowReconnect       bool   `json:"allowReconnect"`       // 是否允许重连
	Username             string `json:"username"`             // 用户名
	Password             string `json:"password"`             // 密码
	Token                string `json:"token"`                // Token 认证
	DisableNoEcho        bool   `json:"disableNoEcho"`        // 禁用 NoEcho
	RetryOnFailedConnect bool   `json:"retryOnFailedConnect"` // 连接失败时重试
}




































