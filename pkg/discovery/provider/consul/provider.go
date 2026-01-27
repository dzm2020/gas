package consul

import (
	"context"
	"errors"
	"fmt"
	"sync"

	discoveryApi "github.com/dzm2020/gas/pkg/discovery"
	"github.com/dzm2020/gas/pkg/discovery/iface"
	"github.com/dzm2020/gas/pkg/lib/grs"
	"github.com/dzm2020/gas/pkg/lib/stopper"

	"github.com/hashicorp/consul/api"
	"github.com/spf13/viper"
)

func init() {
	_ = discoveryApi.GetFactoryMgr().Register("consul", func(args ...any) (iface.IDiscovery, error) {
		if len(args) == 0 {
			return nil, errors.New("consul provider: config is required")
		}
		config, ok := args[0].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("consul provider: config must be map[string]interface{}, got %T", args[0])
		}
		consulCfg := DefaultConfig()
		vp := viper.New()
		vp.Set("", config)
		if err := vp.UnmarshalKey("", consulCfg); err != nil {
			return nil, fmt.Errorf("consul provider: failed to unmarshal config: %w", err)
		}
		return New(consulCfg), nil
	})
}

var _ iface.IDiscovery = (*Provider)(nil)

func New(config *Config) *Provider {
	provider := &Provider{
		config: config,
	}
	provider.ctx, provider.cancel = context.WithCancel(context.Background())
	return provider
}

type Provider struct {
	stopper.Stopper

	client *api.Client

	config *Config

	*discovery // 集群监控
	*registrar // 注册

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (c *Provider) Run(ctx context.Context) error {
	if err := c.connect(); err != nil {
		return err
	}

	c.discovery = newDiscovery(c.ctx, &c.wg, c.client, c.config)
	c.discovery.run()

	c.registrar = newRegistrar(c.ctx, &c.wg, c.client, c.config)
	c.registrar.run()

	return nil
}

func (c *Provider) connect() error {
	cfg := api.DefaultConfig()
	cfg.Address = c.config.Address
	client, err := api.NewClient(cfg)
	if err != nil {
		return err
	}
	c.client = client
	return nil
}

func (c *Provider) Shutdown(ctx context.Context) error {
	if !c.Stop() {
		return nil
	}

	c.cancel()
	c.registrar.Shutdown()

	grs.WaitWithContext(ctx, &c.wg)
	return nil
}
