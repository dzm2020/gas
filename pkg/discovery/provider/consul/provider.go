package consul

import (
	"context"
	discoveryApi "gas/pkg/discovery"
	"gas/pkg/discovery/iface"
	"sync"

	"github.com/hashicorp/consul/api"
	"github.com/spf13/viper"
)

func init() {
	_ = discoveryApi.GetFactoryMgr().Register("consul", func(args ...any) (iface.IDiscovery, error) {
		config := args[0].(map[string]interface{})
		consulCfg := defaultConfig()
		vp := viper.New()
		vp.Set("", config)
		if err := vp.UnmarshalKey("", consulCfg); err != nil {
			return nil, err
		}
		return New(consulCfg), nil
	})
}

var _ iface.IDiscovery = (*Provider)(nil)

func New(config *Config) *Provider {
	return &Provider{
		config: config,
	}
}

type Provider struct {
	*api.Client
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
		return err
	}

	c.discovery = newDiscovery(c)
	c.registrar = newRegistrar(c)

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
	c.Client = client
	return nil
}

func (c *Provider) Shutdown(ctx context.Context) error {
	c.stopOnce.Do(func() {
		if c.cancel != nil {
			c.cancel()
		}
	})
	return nil
}
