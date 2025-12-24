package consul

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gas/pkg/discovery"
	"gas/pkg/discovery/iface"
	"gas/pkg/glog"
	"sync"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

var errConsulNotInitialized = errors.New("provider not initialized, call Run() first")

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
		return nil, fmt.Errorf("invalid consul config: %w", err)
	}

	stopCh := make(chan struct{})
	provider := &Provider{
		config:  config,
		stopCh:  stopCh,
		members: make(map[uint64]*iface.Member),
	}

	return provider, nil
}

type Provider struct {
	client *api.Client
	config *Config

	stopOnce sync.Once
	stopCh   chan struct{}

	serviceWatcher *serviceWatcher
	reg            *registrar
	// 节点存储
	memberMu sync.RWMutex
	members  map[uint64]*iface.Member
}

func (c *Provider) Run(ctx context.Context) error {
	client, err := c.connect()
	if err != nil {
		glog.Error("consul连接失败", zap.String("address", c.config.Address), zap.Error(err))
		return err
	}
	c.client = client
	glog.Info("consul连接成功", zap.String("address", c.config.Address))

	c.serviceWatcher = newServiceWatcher(client, c.config, c.stopCh, c.onNodeChange)
	c.reg = newConsulRegistrar(client, c.config)
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

func (c *Provider) Watch(kind string, listener iface.ServiceChangeListener) error {
	if c.serviceWatcher == nil {
		return errConsulNotInitialized
	}
	watcher := c.serviceWatcher.GetOrCreateWatcher(kind)
	watcher.Add(listener)
	glog.Info("consul监控", zap.String("kind", kind))
	return nil
}

func (c *Provider) Unwatch(kind string, listener iface.ServiceChangeListener) {
	watcher := c.serviceWatcher.GetWatcher(kind)
	if watcher == nil {
		return
	}
	_ = watcher.Remove(listener)
	glog.Info("consul取消监控", zap.String("kind", kind))
}

// onNodeChange 节点变化回调，更新节点存储
func (c *Provider) onNodeChange(topology *iface.Topology) {
	c.memberMu.Lock()
	defer c.memberMu.Unlock()

	for _, member := range topology.Joined {
		if member != nil {
			c.members[member.GetID()] = convertor.DeepClone(member)
		}
	}
	for _, member := range topology.Alive {
		if member != nil {
			c.members[member.GetID()] = convertor.DeepClone(member)
		}
	}
	for _, member := range topology.Left {
		if member == nil {
			continue
		}
		delete(c.members, member.GetID())
	}
}

func (c *Provider) GetById(memberId uint64) *iface.Member {
	c.memberMu.RLock()
	defer c.memberMu.RUnlock()
	if member, exists := c.members[memberId]; exists {
		return convertor.DeepClone(member)
	}
	return nil
}

func (c *Provider) GetByKind(service string) []*iface.Member {
	c.memberMu.RLock()
	defer c.memberMu.RUnlock()

	result := make([]*iface.Member, 0)
	for _, member := range c.members {
		if member.GetKind() == service {
			result = append(result, convertor.DeepClone(member))
		}
	}
	return result
}

func (c *Provider) GetAll() []*iface.Member {
	c.memberMu.RLock()
	defer c.memberMu.RUnlock()

	result := make([]*iface.Member, 0, len(c.members))
	for _, member := range c.members {
		result = append(result, convertor.DeepClone(member))
	}
	return result
}

func (c *Provider) Register(member *iface.Member) error {
	if c.reg == nil {
		return errConsulNotInitialized
	}
	return c.reg.register(member)
}

func (c *Provider) Deregister(memberId uint64) error {
	if c.reg == nil {
		return errConsulNotInitialized
	}
	return c.reg.deregister(memberId)
}

func (c *Provider) Shutdown(ctx context.Context) error {
	c.stopOnce.Do(func() {
		close(c.stopCh)

		// 清理所有注册的成员
		if c.reg != nil {
			c.reg.cleanup()
		}
	})
	return nil
}
