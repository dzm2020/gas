package consul

import (
	"gas/pkg/discovery/iface"
	"sync"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/hashicorp/consul/api"
)

var _ iface.IDiscovery = (*Provider)(nil)

func New(options ...Option) (*Provider, error) {
	opts := loadOptions(options...)
	client, err := newConsulClient(opts.Address)
	if err != nil {
		return nil, err
	}

	stopCh := make(chan struct{})
	provider := &Provider{
		client:          client,
		opts:            opts,
		stopCh:          stopCh,
		watchers:        make(map[string]*consulWatcher),
		consulRegistrar: newConsulRegistrar(client, opts, stopCh),
	}
	return provider, nil
}

type Provider struct {
	client *api.Client
	opts   *Options

	stopOnce sync.Once
	stopCh   chan struct{}

	watchersMu sync.RWMutex
	watchers   map[string]*consulWatcher

	*consulRegistrar
}

func newConsulClient(addr string) (*api.Client, error) {
	cfg := api.DefaultConfig()
	cfg.Address = addr
	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	if _, err = client.Status().Leader(); err != nil {
		return nil, err
	}
	return client, nil
}

func (c *Provider) Watch(service string, handler iface.UpdateHandler) error {
	c.watchersMu.Lock()

	// 如果已经存在该 service 的 watcher，将回调添加到 watcher 中
	if watcher, exists := c.watchers[service]; exists {
		c.watchersMu.Unlock()
		return watcher.WatchNode(service, handler)
	}

	// 为每个 service 创建独立的 watcher

	watcher := newConsulWatcher(c.client, service, c.opts, c.stopCh)
	// 建立 service 和 watcher 的映射
	c.watchers[service] = watcher
	c.watchersMu.Unlock()

	// 开始监听
	if err := watcher.WatchNode(service, handler); err != nil {
		return err
	}

	// 启动 watcher
	watcher.start()
	return nil
}

// Unwatch 移除指定 service 的 handler
// 如果 handler 为 nil，则移除该 service 的所有 handlers 并停止监听
// 返回是否成功移除
func (c *Provider) Unwatch(service string, handler iface.UpdateHandler) bool {
	c.watchersMu.Lock()
	defer c.watchersMu.Unlock()

	watcher, exists := c.watchers[service]
	if !exists {
		return false
	}

	// 如果 handler 为 nil，移除整个 watcher
	if handler == nil {
		delete(c.watchers, service)
		return true
	}

	// 移除指定的 handler
	removed := watcher.RemoveHandler(handler)

	// 如果所有 handlers 都被移除了，删除 watcher
	if watcher.HandlerCount() == 0 {
		delete(c.watchers, service)
	}

	return removed
}

// GetById 从所有 watcher 的合并 nodelist 中获取节点
func (c *Provider) GetById(nodeId uint64) *iface.Node {
	c.watchersMu.RLock()
	defer c.watchersMu.RUnlock()

	// 遍历所有 watcher，查找节点
	for _, watcher := range c.watchers {
		if node := watcher.GetById(nodeId); node != nil {
			return node
		}
	}
	return nil
}

// GetByKind 从所有 watcher 的合并 nodelist 中按标签类型获取节点
func (c *Provider) GetByKind(kind string) []*iface.Node {
	c.watchersMu.RLock()
	defer c.watchersMu.RUnlock()

	result := make([]*iface.Node, 0)
	nodeMap := make(map[uint64]*iface.Node)

	// 合并所有 watcher 的节点
	for _, watcher := range c.watchers {
		nodes := watcher.GetByKind(kind)
		for _, node := range nodes {
			if _, exists := nodeMap[node.GetID()]; !exists {
				nodeMap[node.GetID()] = node
				result = append(result, node)
			}
		}
	}

	return result
}

// GetAll 从所有 watcher 的合并 nodelist 中获取所有节点
func (c *Provider) GetAll() []*iface.Node {
	c.watchersMu.RLock()
	defer c.watchersMu.RUnlock()

	result := make([]*iface.Node, 0)
	nodeMap := make(map[uint64]*iface.Node)

	// 合并所有 watcher 的节点
	for _, watcher := range c.watchers {
		nodes := watcher.GetAll()
		for _, node := range nodes {
			if _, exists := nodeMap[node.GetID()]; !exists {
				nodeMap[node.GetID()] = node
				result = append(result, convertor.DeepClone(node))
			}
		}
	}

	return result
}

func (c *Provider) Close() error {
	c.stopOnce.Do(func() {
		close(c.stopCh)
		c.consulRegistrar.shutdown()
	})

	// 等待所有 watcher 完成
	c.watchersMu.RLock()
	for _, watcher := range c.watchers {
		watcher.Wait()
	}
	c.watchersMu.RUnlock()

	c.consulRegistrar.Wait()
	return nil
}
