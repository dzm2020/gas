package consul

import (
	"context"
	"gas/pkg/discovery/iface"
	"gas/pkg/glog"
	"gas/pkg/lib/grs"
	"sync"
	"time"

	"github.com/duke-git/lancet/v2/maputil"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

// serviceWatcher 服务列表监听器，负责监听 Consul 服务列表变化并管理 watchers
type serviceWatcher struct {
	ctx       context.Context
	client    *api.Client
	config    *Config
	waitIndex uint64
	mu        sync.RWMutex
	watchers  map[string]*Watcher
}

// newServiceWatcher 创建服务列表监听器
func newServiceWatcher(ctx context.Context, client *api.Client, config *Config) *serviceWatcher {
	s := &serviceWatcher{
		ctx:       ctx,
		client:    client,
		config:    config,
		waitIndex: 0,
		watchers:  make(map[string]*Watcher),
	}
	grs.Go(func(ctx context.Context) {
		s.watch()
	})
	return s
}

// watch 持续监听服务列表变化
func (sw *serviceWatcher) watch() {
	for {
		select {
		case <-sw.ctx.Done():
			return
		default:
			if err := sw.fetch(); err != nil {
				glog.Error("consul获取服务列表失败", zap.Error(err))
				select {
				case <-sw.ctx.Done():
					return
				case <-time.After(time.Second):
				}
			}
		}
	}
}

// fetch 获取服务列表并更新 watchers
func (sw *serviceWatcher) fetch() error {
	options := &api.QueryOptions{
		WaitIndex: sw.waitIndex,
		WaitTime:  sw.config.WatchWaitTime,
	}
	options = options.WithContext(sw.ctx)
	services, meta, err := sw.client.Catalog().Services(options)
	if err != nil {
		return err
	}

	sw.waitIndex = meta.LastIndex

	// 创建新的服务 watcher
	for name := range services {
		if name == "consul" {
			continue
		}
		_ = sw.getOrCreateWatcher(name)
	}

	// 清理已删除的服务 watcher
	for name := range sw.watchers {
		if _, exists := services[name]; exists {
			continue
		}
		delete(sw.watchers, name)
	}

	return nil
}

func (sw *serviceWatcher) getWatcher(name string) *Watcher {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	return sw.watchers[name]
}

func (sw *serviceWatcher) addWatcher(name string, watcher *Watcher) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.watchers[name] = watcher
}

func (sw *serviceWatcher) delWatcher(name string) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	delete(sw.watchers, name)
}

func (sw *serviceWatcher) rangeWatcher(f func(watcher *Watcher) bool) {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	for _, watcher := range sw.watchers {
		if !f(watcher) {
			return
		}
	}
}

func (sw *serviceWatcher) getOrCreateWatcher(name string) *Watcher {
	var ok bool
	watcher := sw.getWatcher(name)
	if watcher != nil {
		return watcher
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()
	if watcher, ok = sw.watchers[name]; ok {
		return watcher
	}

	watcher = newWatcher(sw.ctx, sw.client, name, sw.config)
	sw.watchers[name] = watcher
	return watcher
}

func (sw *serviceWatcher) Watch(kind string, listener iface.ServiceChangeListener) {
	watcher := sw.getOrCreateWatcher(kind)
	watcher.Add(listener)
}

func (sw *serviceWatcher) Unwatch(kind string, listener iface.ServiceChangeListener) {
	watcher := sw.getWatcher(kind)
	if watcher == nil {
		return
	}
	watcher.Remove(listener)
}

func (sw *serviceWatcher) GetByKind(kind string) map[uint64]*iface.Member {
	watcher := sw.getWatcher(kind)
	if watcher == nil {
		return nil
	}
	return watcher.GetAll()
}

func (sw *serviceWatcher) GetAll() map[uint64]*iface.Member {
	var result = make(map[uint64]*iface.Member)
	sw.rangeWatcher(func(watcher *Watcher) bool {
		result = maputil.Merge(result, watcher.GetAll())
		return true
	})
	return result
}

func (sw *serviceWatcher) GetById(memberId uint64) *iface.Member {
	var result *iface.Member
	sw.rangeWatcher(func(watcher *Watcher) bool {
		result = watcher.GetById(memberId)
		return result == nil
	})
	return result
}
