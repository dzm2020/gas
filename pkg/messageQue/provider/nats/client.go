package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"gas/pkg/lib/glog"
	"gas/pkg/messageQue"
	"gas/pkg/messageQue/iface"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func init() {
	_ = messageQue.Register("nats", func(configData json.RawMessage) (iface.IMessageQue, error) {
		natsCfg := defaultConfig()
		// 如果是从 Options 字段传入的 JSON 数据，尝试解析
		if len(configData) > 0 {
			if err := json.Unmarshal(configData, natsCfg); err != nil {
				return nil, fmt.Errorf("failed to parse nats config: %w", err)
			}
		}
		return New(natsCfg.Servers, natsCfg), nil
	})
}

func New(servers []string, cfg *Config) *Client {
	if cfg == nil {
		cfg = defaultConfig()
	}
	return &Client{
		servers:       servers,
		cfg:           cfg,
		subscriptions: make(map[iface.ISubscription]struct{}),
	}
}

type Client struct {
	servers       []string
	cfg           *Config
	conn          *nats.Conn
	mu            sync.RWMutex
	subscriptions map[iface.ISubscription]struct{}
}

func (n *Client) Run(ctx context.Context) error {
	natsOpts := buildNatsOptions(n.cfg)
	client, err := nats.Connect(strings.Join(n.servers, ","), natsOpts...)
	if err != nil {
		glog.Error("NATS连接失败", zap.Strings("servers", n.servers), zap.Error(err))
		return err
	}
	if client.Status() != nats.CONNECTED {
		glog.Error("NATS连接失败", zap.Strings("servers", n.servers), zap.Error(err))
		return fmt.Errorf("nats connect failed: %s", client.Status().String())
	}
	n.conn = client
	glog.Info("NATS连接成功", zap.Strings("servers", n.servers))
	return nil
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
		glog.Error("NATS订阅主题失败", zap.String("subject", subject), zap.Error(err))
		return nil, err
	}
	subscription := &Subscription{sub: sub}

	n.mu.Lock()
	n.subscriptions[subscription] = struct{}{}
	n.mu.Unlock()

	glog.Info("NATS订阅主题成功", zap.String("subject", subject))
	return subscription, nil
}

// Request 实现 Cluster 接口的 Request 方法
func (n *Client) Request(subject string, data []byte, timeout time.Duration) ([]byte, error) {
	reply, err := n.conn.Request(subject, data, timeout)
	if err != nil {
		glog.Error("NATS发送消息失败", zap.String("subject", subject), zap.Error(err))
		return nil, err
	}
	glog.Debug("NATS发送消息", zap.String("subject", subject), zap.Error(err))
	return reply.Data, nil
}

func (n *Client) removeSub(subscription iface.ISubscription) {
	n.mu.Lock()
	delete(n.subscriptions, subscription)
	n.mu.Unlock()
}

func (n *Client) Shutdown(ctx context.Context) error {
	// 关闭所有 subscription
	var subscriptions []iface.ISubscription
	n.mu.RLock()
	for sub, _ := range n.subscriptions {
		subscriptions = append(subscriptions, sub)
	}
	n.mu.RUnlock()
	for _, sub := range subscriptions {
		_ = sub.Unsubscribe()
	}
	n.conn.Close()
	glog.Info("NAT客户端关闭")
	return nil
}

// Subscription 实现 iface.ISubscription 接口
type Subscription struct {
	sub *nats.Subscription
	c   *Client
}

// Unsubscribe 实现 Subscription 接口
func (n *Subscription) Unsubscribe() error {
	glog.Info("NAT取消订阅", zap.String("subject", n.sub.Subject))
	if n.c == nil {
		return nil
	}
	n.c.removeSub(n)
	return n.sub.Unsubscribe()
}
