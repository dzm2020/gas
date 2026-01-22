package nats

import (
	"context"
	"time"

	"github.com/dzm2020/gas/pkg/lib/stopper"
	"github.com/dzm2020/gas/pkg/lib/xerror"
	"github.com/dzm2020/gas/pkg/messageQue"
	"github.com/dzm2020/gas/pkg/messageQue/iface"

	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

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
	stopper.Stopper
	cfg     *Config
	pool    *ConnPool  // 连接池，用于 Publish 和 Request
	subConn *nats.Conn // 专门的订阅连接
}

func (n *Client) Run(ctx context.Context) (err error) {
	n.pool = NewPool(n.cfg)
	n.subConn, err = n.pool.get()
	return
}

func (n *Client) Subscribe(subject string, subscriber iface.ISubscriber) (iface.ISubscription, error) {
	conn := n.subConn
	return conn.Subscribe(subject, func(m *nats.Msg) {
		response := func(data []byte) error {
			if m.Reply == "" {
				return nil
			}
			return m.Respond(data)
		}
		subscriber.OnMessage(m.Data, response)
	})
}

func (n *Client) Publish(subject string, data []byte) error {
	conn, err := n.pool.get()
	if err != nil {
		return xerror.Wrapf(err, "从连接池获取连接失败, subject:%s", subject)
	}
	defer n.pool.put(conn)

	return conn.Publish(subject, data)
}

func (n *Client) Request(subject string, data []byte, timeout time.Duration) ([]byte, error) {
	conn, err := n.pool.get()
	if err != nil {
		return nil, xerror.Wrapf(err, "从连接池获取连接失败, subject:%s", subject)
	}
	defer n.pool.put(conn)

	ret, err := conn.Request(subject, data, timeout)
	if err != nil {
		return nil, xerror.Wrapf(err, "subject:%s", subject)
	}
	return ret.Data, nil
}

func (n *Client) Shutdown(ctx context.Context) error {
	if !n.Stop() {
		return nil
	}
	if n.subConn != nil && !n.subConn.IsClosed() {
		n.subConn.Close()
		n.subConn = nil
	}
	// 关闭连接池
	if n.pool != nil {
		n.pool.close()
	}
	return nil
}
