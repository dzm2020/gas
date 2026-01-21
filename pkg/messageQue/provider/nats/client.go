package nats

import (
	"context"
	"github.com/dzm2020/gas/pkg/lib/xerror"
	"github.com/dzm2020/gas/pkg/messageQue"
	"github.com/dzm2020/gas/pkg/messageQue/iface"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

// todo 增加连接池

func init() {
	_ = messageQue.GetFactoryMgr().Register("nats", func(args ...any) (iface.IMessageQue, error) {
		config := args[0].(map[string]interface{})

		natsCfg := defaultConfig()
		vp := viper.New()
		vp.Set("", config)
		if err := vp.UnmarshalKey("", natsCfg); err != nil {
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
