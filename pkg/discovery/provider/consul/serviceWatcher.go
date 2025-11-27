package consul

import (
	"gas/pkg/discovery/iface"
	"gas/pkg/lib/glog"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

// serviceWatcher 服务列表监听器，负责监听 Consul 服务列表变化并管理 watchers
type serviceWatcher struct {
	client    *api.Client
	opts      *Options
	stopCh    <-chan struct{}
	waitIndex uint64
	wg        sync.WaitGroup

	// watchers 管理
	watchersMu sync.RWMutex
	watchers   map[string]*consulWatcher

	// 节点变化回调
	onNodeChange iface.ServiceChangeListener
}

// newServiceWatcher 创建服务列表监听器
func newServiceWatcher(
	client *api.Client,
	opts *Options,
	stopCh <-chan struct{},
	onNodeChange iface.ServiceChangeListener,
) *serviceWatcher {
	s := &serviceWatcher{
		client:       client,
		opts:         opts,
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

	sw.wg.Add(1)
	go sw.watch()
}

// watch 持续监听服务列表变化
func (sw *serviceWatcher) watch() {
	defer func() {
		if rec := recover(); rec != nil {
			glog.Error("consul serviceWatcher watch panic", zap.Any("error", rec))
		}
		sw.wg.Done()
		glog.Info("consul serviceWatcher close")
	}()

	glog.Info("consul serviceWatcher start")

	for {
		select {
		case <-sw.stopCh:
			return
		default:
			if err := sw.fetch(); err != nil {
				glog.Error("consul serviceWatcher fetch services failed", zap.Error(err))
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
func (sw *serviceWatcher) fetch() error {
	services, meta, err := sw.client.Catalog().Services(&api.QueryOptions{
		WaitIndex: sw.waitIndex,
		WaitTime:  sw.opts.WatchWaitTime,
	})
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
			watcher := newConsulWatcher(sw.client, name, sw.opts, sw.stopCh)
			sw.watchers[name] = watcher
			if sw.onNodeChange != nil {
				watcher.Add(sw.onNodeChange)
			}
			watcher.start()
			glog.Info("consul serviceWatcher: created watcher for new service", zap.String("service", name))
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
		glog.Debug("consul serviceWatcher: using existing watcher", zap.String("service", service))
		return watcher
	}

	glog.Debug("consul serviceWatcher: creating new watcher", zap.String("service", service))
	watcher := newConsulWatcher(sw.client, service, sw.opts, sw.stopCh)
	sw.watchers[service] = watcher
	if sw.onNodeChange != nil {
		watcher.Add(sw.onNodeChange)
	}
	watcher.start()
	return watcher
}

// WaitAllWatchers 等待所有 watchers 完成
func (sw *serviceWatcher) WaitAllWatchers() {
	sw.watchersMu.RLock()
	watchers := make([]*consulWatcher, 0, len(sw.watchers))
	for _, watcher := range sw.watchers {
		watchers = append(watchers, watcher)
	}
	sw.watchersMu.RUnlock()

	for _, watcher := range watchers {
		watcher.Wait()
	}
}

// Wait 等待监听 goroutine 完成
func (sw *serviceWatcher) Wait() {
	sw.wg.Wait()
	sw.WaitAllWatchers()
}
