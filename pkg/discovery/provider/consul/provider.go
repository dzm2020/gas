package consul

import (
	"context"
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
	provider := &Provider{
		config: config,
	}
	provider.ctx, provider.cancel = context.WithCancel(context.Background())
	return provider
}

type Provider struct {
	stopper.Stopper
	*api.Client
	config *Config

	*discovery // 集群监控
	*registrar // 注册

	ctx    context.Context
	cancel context.CancelFunc

	wg sync.WaitGroup
}

func (c *Provider) Run(ctx context.Context) error {

	if err := c.connect(); err != nil {
		return err
	}

	c.discovery = newDiscovery(c.ctx, &c.wg, c.Client, c.config)
	c.discovery.run()
	c.registrar = newRegistrar(c.ctx, &c.wg, c.Client, c.config)

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
	if !c.Stop() {
		return nil
	}

	c.cancel()

	grs.WaitWithContext(ctx, &c.wg)
	return nil
}
