package consul

import (
	"context"
	"encoding/json"
	"fmt"
	"gas/pkg/discovery"
	"gas/pkg/discovery/iface"
	"gas/pkg/glog"
	"sync"

	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

func init() {
	_ = discovery.Register("consul", func(configData json.RawMessage) (iface.IDiscovery, error) {
		consulCfg := defaultConfig()
		if len(configData) > 0 {
			if err := json.Unmarshal(configData, consulCfg); err != nil {
				return nil, fmt.Errorf("failed to parse consul config: %w", err)
			}
		}
		return New(consulCfg)
	})
}

var _ iface.IDiscovery = (*Provider)(nil)

func New(config *Config) (*Provider, error) {
	if config == nil {
		config = defaultConfig()
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}

	provider := &Provider{
		config: config,
	}

	return provider, nil
}

type Provider struct {
	client *api.Client
	config *Config

	*serviceWatcher
	*registrar

	stopOnce sync.Once

	ctx    context.Context
	cancel context.CancelFunc
}

func (c *Provider) Run(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)

	client, err := c.connect()
	if err != nil {
		glog.Error("consul连接失败", zap.String("address", c.config.Address), zap.Error(err))
		return err
	}
	c.client = client

	c.serviceWatcher = newServiceWatcher(c.ctx, client, c.config)
	c.registrar = newConsulRegistrar(client, c.config)

	glog.Info("consul连接成功", zap.String("address", c.config.Address))
	return nil
}

func (c *Provider) connect() (*api.Client, error) {
	cfg := api.DefaultConfig()
	cfg.Address = c.config.Address
	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	if _, err = client.Status().Leader(); err != nil {
		return nil, err
	}
	return client, nil
}

func (c *Provider) Shutdown(ctx context.Context) error {
	c.stopOnce.Do(func() {
		c.cancel()
		// 清理所有注册的成员
		if c.registrar != nil {
			c.registrar.cleanup()
		}
	})
	return nil
}
