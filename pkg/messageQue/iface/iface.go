package iface

import "time"

// IMessageQue 集群通信接口，定义核心通信能力
type IMessageQue interface {
	// Publish 向指定主题发布消息（无回复）
	Publish(subject string, data []byte) error
	// Subscribe 订阅主题，接收消息（非阻塞，通过回调处理）
	Subscribe(subject string, handler MsgHandler) (ISubscription, error)
	// Request 发送请求并等待回复（同步 RPC 模式）
	Request(subject string, data []byte, timeout time.Duration) ([]byte, error)
	// Close 关闭集群连接
	Close() error
}

// ISubscription 订阅关系接口，用于取消订阅
type ISubscription interface {
	Unsubscribe() error
}

// MsgHandler 消息处理函数类型
type MsgHandler func(request []byte, response func([]byte) error)
