package consul

import (
	"context"
	"encoding/json"
	"fmt"
	discoveryApi "gas/pkg/discovery"
	"gas/pkg/discovery/iface"
	"gas/pkg/glog"
	"sync"

	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

func init() {
	_ = discoveryApi.Register("consul", func(configData json.RawMessage) (iface.IDiscovery, error) {
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

	*discovery // 集群监控
	*registrar // 注册

	stopOnce sync.Once

	ctx    context.Context
	cancel context.CancelFunc
}

func (c *Provider) Run(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)

	if err := c.connect(); err != nil {
		glog.Error("consul连接失败", zap.String("address", c.config.Address), zap.Error(err))
		return err
	}

	c.discovery = newDiscovery(c.ctx, c.client, c.config)
	c.registrar = newRegistrar(c.ctx, c.client, c.config)

	glog.Info("consul连接成功", zap.String("address", c.config.Address))
	return nil
}

func (c *Provider) connect() error {
	cfg := api.DefaultConfig()
	cfg.Address = c.config.Address
	client, err := api.NewClient(cfg)
	if err != nil {
		return err
	}
	if _, err = client.Status().Leader(); err != nil {
		return err
	}
	c.client = client
	return nil
}

func (c *Provider) Shutdown(ctx context.Context) error {
	c.stopOnce.Do(func() {
		c.cancel()
	})
	return nil
}
