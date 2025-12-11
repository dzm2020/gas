package consul

import (
	"context"
	"gas/pkg/discovery/iface"
	"gas/pkg/lib"
	"gas/pkg/lib/glog"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

// serviceWatcher 服务列表监听器，负责监听 Consul 服务列表变化并管理 watchers
type serviceWatcher struct {
	client    *api.Client
	config    *Config
	stopCh    <-chan struct{}
	waitIndex uint64

	// watchers 管理
	watchersMu sync.RWMutex
	watchers   map[string]*consulWatcher

	// 节点变化回调
	onNodeChange iface.ServiceChangeListener
}

// newServiceWatcher 创建服务列表监听器
func newServiceWatcher(
	client *api.Client,
	config *Config,
	stopCh <-chan struct{},
	onNodeChange iface.ServiceChangeListener,
) *serviceWatcher {
	s := &serviceWatcher{
		client:       client,
		config:       config,
		stopCh:       stopCh,
		waitIndex:    0,
		watchers:     make(map[string]*consulWatcher),
		onNodeChange: onNodeChange,
	}
	s.Start()
	return s
}

// Start 启动服务列表监听
func (sw *serviceWatcher) Start() {
	lib.Go(func(ctx context.Context) {
		sw.watch(ctx)
	})
}

// watch 持续监听服务列表变化
func (sw *serviceWatcher) watch(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-sw.stopCh:
			return
		default:
			if err := sw.fetch(ctx); err != nil {
				glog.Error("Consul获取服务列表失败", zap.Error(err))
				select {
				case <-time.After(time.Second):
				case <-sw.stopCh:
					return
				}
			}
		}
	}
}

// fetch 获取服务列表并更新 watchers
func (sw *serviceWatcher) fetch(ctx context.Context) error {
	options := &api.QueryOptions{
		WaitIndex: sw.waitIndex,
		WaitTime:  sw.config.WatchWaitTime,
	}
	options = options.WithContext(ctx)
	services, meta, err := sw.client.Catalog().Services(options)
	if err != nil {
		return err
	}

	sw.waitIndex = meta.LastIndex

	sw.watchersMu.Lock()
	defer sw.watchersMu.Unlock()

	for name := range services {
		if name == "consul" {
			continue
		}
		if _, exists := sw.watchers[name]; !exists {
			watcher := newConsulWatcher(sw.client, name, sw.config, sw.stopCh)
			sw.watchers[name] = watcher
			if sw.onNodeChange != nil {
				watcher.Add(sw.onNodeChange)
			}
			watcher.start()
		}
	}
	return nil
}

// GetWatcher 获取指定服务的 watcher
func (sw *serviceWatcher) GetWatcher(service string) *consulWatcher {
	sw.watchersMu.RLock()
	defer sw.watchersMu.RUnlock()
	return sw.watchers[service]
}

// GetOrCreateWatcher 获取或创建指定服务的 watcher
func (sw *serviceWatcher) GetOrCreateWatcher(service string) *consulWatcher {
	sw.watchersMu.Lock()
	defer sw.watchersMu.Unlock()

	if watcher, exists := sw.watchers[service]; exists {
		return watcher
	}

	watcher := newConsulWatcher(sw.client, service, sw.config, sw.stopCh)
	sw.watchers[service] = watcher
	if sw.onNodeChange != nil {
		watcher.Add(sw.onNodeChange)
	}
	watcher.start()
	return watcher
}
