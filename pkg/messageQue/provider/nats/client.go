package nats

import (
	"context"
	"strings"
	"time"

	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/stopper"
	"github.com/dzm2020/gas/pkg/lib/xerror"
	"github.com/dzm2020/gas/pkg/messageQue/iface"
	"github.com/dzm2020/gas/pkg/messageQue/registry"
	"go.uber.org/zap"

	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

func init() {
	_ = registry.GetFactoryMgr().Register("nats", func(args ...any) (iface.IMessageQue, error) {
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
	pool    *ConnPool  // 按 subject 固定连接，同一 subject 的 Publish/Request 用同一 conn 保证顺序
	subConn *nats.Conn // 专门的订阅连接
}

func (n *Client) Run(ctx context.Context) (err error) {
	n.pool = NewPool(n.cfg)
	n.subConn, err = nats.Connect(strings.Join(n.cfg.Servers, ","), toOptions(n.cfg)...)
	if err != nil {
		return err
	}
	glog.Debug("NATS启动成功", zap.Strings("address", n.cfg.Servers))
	return nil
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
	conn, err := n.pool.getConnBySubject(subject)
	if err != nil {
		return xerror.Wrapf(err, "从连接池获取连接失败, subject:%s", subject)
	}
	return conn.Publish(subject, data)
}

func (n *Client) Request(subject string, data []byte, timeout time.Duration) ([]byte, error) {
	conn, err := n.pool.getConnBySubject(subject)
	if err != nil {
		return nil, xerror.Wrapf(err, "从连接池获取连接失败, subject:%s", subject)
	}
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

	glog.Debug("NATS关闭", zap.Strings("address", n.cfg.Servers))
	return nil
}
