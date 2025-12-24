package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"gas/pkg/glog"
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
				return nil, fmt.Errorf("解析nats配置文件失败: %w", err)
			}
		}

		if err := natsCfg.Validate(); err != nil {
			return nil, fmt.Errorf("验证nats配置文件失败: %w", err)
		}

		return New(natsCfg), nil
	})
}

func New(cfg *Config) *Client {
	if cfg == nil {
		cfg = defaultConfig()
	}
	return &Client{
		servers:       cfg.Servers,
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
	runOnce       sync.Once
	shutdownOnce  sync.Once
}

func (n *Client) Run(ctx context.Context) error {
	var err error
	n.runOnce.Do(func() {
		natsOpts := toOptions(n.cfg)
		client, connectErr := nats.Connect(strings.Join(n.servers, ","), natsOpts...)
		if connectErr != nil {
			glog.Error("NATS连接失败", zap.Strings("servers", n.servers), zap.Error(connectErr))
			err = connectErr
			return
		}
		if client.Status() != nats.CONNECTED {
			err = fmt.Errorf("nats connect failed: %s", client.Status().String())
			glog.Error("NATS连接失败", zap.Strings("servers", n.servers), zap.Error(err))
			client.Close()
			return
		}
		n.mu.Lock()
		n.conn = client
		n.mu.Unlock()
		glog.Info("NATS连接成功", zap.Strings("servers", n.servers))
	})
	return err
}

func (n *Client) Publish(subject string, data []byte) error {
	n.mu.RLock()
	conn := n.conn
	n.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("NATS client not connected")
	}
	return conn.Publish(subject, data)
}

func (n *Client) Subscribe(subject string, subscriber iface.ISubscriber) (iface.ISubscription, error) {
	n.mu.RLock()
	conn := n.conn
	n.mu.RUnlock()
	if conn == nil {
		return nil, fmt.Errorf("NATS client not connected")
	}

	sub, err := conn.Subscribe(subject, func(m *nats.Msg) {
		bytes, err := subscriber.OnMessage(m.Data)
		if err != nil {
			glog.Error("NATS处理消息失败", zap.String("subject", subject), zap.Error(err))
			return
		}
		if m.Reply == "" {
			return
		}
		if err = m.Respond(bytes); err != nil {
			glog.Error("NATS回复消息失败", zap.String("subject", subject), zap.Error(err))
		}
	})
	if err != nil {
		glog.Error("NATS订阅主题失败", zap.String("subject", subject), zap.Error(err))
		return nil, err
	}

	subscription := &Subscription{sub: sub, c: n}
	n.mu.Lock()
	n.subscriptions[subscription] = struct{}{}
	n.mu.Unlock()

	glog.Info("NATS订阅主题成功", zap.String("subject", subject))
	return subscription, nil
}

func (n *Client) Request(subject string, data []byte, timeout time.Duration) ([]byte, error) {
	n.mu.RLock()
	conn := n.conn
	n.mu.RUnlock()
	if conn == nil {
		return nil, fmt.Errorf("NATS client not connected")
	}

	reply, err := conn.Request(subject, data, timeout)
	if err != nil {
		glog.Error("NATS发送消息失败", zap.String("subject", subject), zap.Error(err))
		return nil, err
	}
	glog.Debug("NATS发送消息", zap.String("subject", subject))
	return reply.Data, nil
}

func (n *Client) removeSub(subscription iface.ISubscription) {
	n.mu.Lock()
	delete(n.subscriptions, subscription)
	n.mu.Unlock()
}

func (n *Client) Shutdown(ctx context.Context) error {
	var lastErr error
	n.shutdownOnce.Do(func() {
		var subscriptions []iface.ISubscription
		n.mu.RLock()
		for sub := range n.subscriptions {
			subscriptions = append(subscriptions, sub)
		}
		n.mu.RUnlock()

		for _, sub := range subscriptions {
			if err := sub.Unsubscribe(); err != nil {
				lastErr = err
			}
		}

		n.mu.Lock()
		conn := n.conn
		n.conn = nil
		n.mu.Unlock()
		if conn != nil {
			conn.Close()
			glog.Info("NATS客户端关闭")
		}
	})
	return lastErr
}

// Subscription 实现 iface.ISubscription 接口
type Subscription struct {
	sub *nats.Subscription
	c   *Client
}

// Unsubscribe 实现 Subscription 接口
func (n *Subscription) Unsubscribe() error {
	if n.c != nil {
		n.c.removeSub(n)
	}
	if n.sub != nil {
		return n.sub.Unsubscribe()
	}
	return nil
}
