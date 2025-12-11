package consul

import (
	"encoding/json"
	"fmt"
	"gas/pkg/discovery"
	"gas/pkg/discovery/iface"
	"gas/pkg/lib/glog"
	"sync"

	"github.com/duke-git/lancet/v2/convertor"
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
	client, err := newConsulClient(config.Address)
	if err != nil {
		return nil, err
	}
	stopCh := make(chan struct{})
	provider := &Provider{
		client:          client,
		config:          config,
		stopCh:          stopCh,
		nodes:           make(map[uint64]*iface.Node),
		consulRegistrar: newConsulRegistrar(client, config, stopCh),
	}
	provider.serviceWatcher = newServiceWatcher(client, config, stopCh, provider.onNodeChange)
	return provider, nil
}

type Provider struct {
	client *api.Client
	config *Config

	stopOnce sync.Once
	stopCh   chan struct{}

	serviceWatcher *serviceWatcher

	// 节点存储
	nodesMu sync.RWMutex
	nodes   map[uint64]*iface.Node

	*consulRegistrar
}

func newConsulClient(addr string) (*api.Client, error) {
	cfg := api.DefaultConfig()
	cfg.Address = addr
	client, err := api.NewClient(cfg)
	if err != nil {
		glog.Error("Consul创建客户端失败", zap.String("address", addr), zap.Error(err))
		return nil, err
	}
	if _, err = client.Status().Leader(); err != nil {
		glog.Error("Consul连接Leader失败", zap.String("address", addr), zap.Error(err))
		return nil, err
	}
	glog.Info("Consul连接成功", zap.String("address", addr))
	return client, nil
}

func (c *Provider) Subscribe(service string, listener iface.ServiceChangeListener) error {
	watcher := c.serviceWatcher.GetOrCreateWatcher(service)
	watcher.Add(listener)
	glog.Info("Consul订阅服务", zap.String("service", service))
	return nil
}

func (c *Provider) Unsubscribe(service string, listener iface.ServiceChangeListener) {
	watcher := c.serviceWatcher.GetWatcher(service)
	if watcher == nil {
		return
	}
	_ = watcher.Remove(listener)
	glog.Info("Consul取消服务订阅", zap.String("service", service))
}

// onNodeChange 节点变化回调，更新节点存储
func (c *Provider) onNodeChange(topology *iface.Topology) {
	c.nodesMu.Lock()
	defer c.nodesMu.Unlock()

	for _, node := range topology.Joined {
		if node != nil {
			c.nodes[node.GetID()] = convertor.DeepClone(node)
		}
	}
	for _, node := range topology.Alive {
		if node != nil {
			c.nodes[node.GetID()] = convertor.DeepClone(node)
		}
	}
	for _, node := range topology.Left {
		if node != nil {
			delete(c.nodes, node.GetID())
		}
	}
}

func (c *Provider) GetById(nodeId uint64) *iface.Node {
	c.nodesMu.RLock()
	defer c.nodesMu.RUnlock()
	if node, exists := c.nodes[nodeId]; exists {
		return convertor.DeepClone(node)
	}
	return nil
}

func (c *Provider) GetService(service string) []*iface.Node {
	c.nodesMu.RLock()
	defer c.nodesMu.RUnlock()

	result := make([]*iface.Node, 0)
	for _, node := range c.nodes {
		if node.GetName() == service {
			result = append(result, convertor.DeepClone(node))
		}
	}
	return result
}

func (c *Provider) GetAll() []*iface.Node {
	c.nodesMu.RLock()
	defer c.nodesMu.RUnlock()

	result := make([]*iface.Node, 0, len(c.nodes))
	for _, node := range c.nodes {
		result = append(result, convertor.DeepClone(node))
	}
	return result
}

func (c *Provider) Close() error {
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})
	return nil
}
