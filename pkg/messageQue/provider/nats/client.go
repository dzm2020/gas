package nats

import (
	"fmt"
	"gas/pkg/messageQue/iface"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// New 创建 NATS 集群通信实例
func New(servers []string, opts ...Option) (*Client, error) {
	options := loadOptions(opts...)
	natsOpts := buildNatsOptions(options)
	// 连接 NATS 集群
	client, err := nats.Connect(strings.Join(servers, ","), natsOpts...)
	if err != nil {
		return nil, err
	}
	if client.Status() != nats.CONNECTED {
		return nil, fmt.Errorf("nats connect failed: %s", client.Status().String())
	}

	return &Client{
		conn: client,
	}, nil
}

// NewWithNatsOptions 使用原生 nats.Option 创建 NATS 集群通信实例（向后兼容）
func NewWithNatsOptions(servers []string, natsOpts ...nats.Option) (*Client, error) {
	conn, err := nats.Connect(strings.Join(servers, ","), natsOpts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		conn: conn,
	}, nil
}

// Client 实现 Cluster 接口的 NATS 具体实现
type Client struct {
	conn *nats.Conn
}

// Publish 实现 Cluster 接口的 Publish 方法
func (n *Client) Publish(subject string, data []byte) error {
	return n.conn.Publish(subject, data)
}

// Subscribe 实现 Cluster 接口的 Subscribe 方法
func (n *Client) Subscribe(subject string, handler iface.MsgHandler) (iface.ISubscription, error) {
	sub, err := n.conn.Subscribe(subject, func(m *nats.Msg) {
		// 包装回复函数：通过 m.Respond 发送回复
		replyFunc := func(replyData []byte) error {

			return m.Respond(replyData)
		}
		if m.Reply == "" {
			replyFunc = nil // 无回复主题时忽略
		}
		handler(m.Data, replyFunc)
	})
	if err != nil {
		return nil, err
	}
	return &Subscription{sub: sub}, nil
}

// Request 实现 Cluster 接口的 Request 方法
func (n *Client) Request(subject string, data []byte, timeout time.Duration) ([]byte, error) {
	reply, err := n.conn.Request(subject, data, timeout)
	if err != nil {
		return nil, err
	}
	return reply.Data, nil
}

// Close 实现 Cluster 接口的 Close 方法
func (n *Client) Close() error {
	n.conn.Close()
	return nil
}

// Subscription 实现 iface.ISubscription 接口
type Subscription struct {
	sub *nats.Subscription
}

// Unsubscribe 实现 Subscription 接口
func (n *Subscription) Unsubscribe() error {
	return n.sub.Unsubscribe()
}












