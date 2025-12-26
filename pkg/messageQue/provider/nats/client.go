package nats

import (
	"context"
	"encoding/json"
	"gas/pkg/lib/xerror"
	"gas/pkg/messageQue"
	"gas/pkg/messageQue/iface"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// todo 增加连接池

func init() {
	_ = messageQue.Register("nats", func(configData json.RawMessage) (iface.IMessageQue, error) {
		natsCfg := defaultConfig()
		// 如果是从 Options 字段传入的 JSON 数据，尝试解析
		if len(configData) > 0 {
			if err := json.Unmarshal(configData, natsCfg); err != nil {
				return nil, err
			}
		}

		if err := natsCfg.Validate(); err != nil {
			return nil, err
		}
		return New(natsCfg), nil
	})
}

func New(cfg *Config) *Client {
	if cfg == nil {
		cfg = defaultConfig()
	}
	return &Client{
		cfg: cfg,
	}
}

type Client struct {
	cfg *Config
	*nats.Conn
}

func (n *Client) Run(ctx context.Context) (err error) {
	servers := n.cfg.Servers
	natsOpts := toOptions(n.cfg)
	n.Conn, err = nats.Connect(strings.Join(servers, ","), natsOpts...)
	return
}

func (n *Client) Subscribe(subject string, subscriber iface.ISubscriber) (iface.ISubscription, error) {
	return n.Conn.Subscribe(subject, func(m *nats.Msg) {
		response := func(data []byte) error {
			if m.Reply == "" {
				return nil
			}
			return m.Respond(data)
		}
		subscriber.OnMessage(m.Data, response)
	})
}

func (n *Client) Request(subject string, data []byte, timeout time.Duration) ([]byte, error) {
	ret, err := n.Conn.Request(subject, data, timeout)
	if err != nil {
		return nil, xerror.Wrapf(err, "subject:%s", subject)
	}
	return ret.Data, nil
}

func (n *Client) Shutdown(ctx context.Context) error {
	n.Close()
	return nil
}
