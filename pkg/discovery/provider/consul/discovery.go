package consul

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dzm2020/gas/pkg/discovery/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/grs"
	"github.com/dzm2020/gas/pkg/lib/stopper"

	"github.com/duke-git/lancet/v2/maputil"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

// discovery 服务列表监听器，负责监听 Consul 服务列表变化并管理 watchers
type discovery struct {
	stopper.Stopper

	client *api.Client
	config *Config

	waitIndex uint64
	mu        sync.RWMutex
	watchers  map[string]*Watcher

	ctx    context.Context
	cancel context.CancelFunc

	wg *sync.WaitGroup
}

// newDiscovery 创建服务列表监听器
func newDiscovery(ctx context.Context, wg *sync.WaitGroup, client *api.Client, config *Config) *discovery {
	d := &discovery{
		client:    client,
		config:    config,
		wg:        wg,
		waitIndex: 0,
		watchers:  make(map[string]*Watcher),
	}

	d.ctx, d.cancel = context.WithCancel(ctx)

	return d
}

func (d *discovery) run() {
	d.wg.Add(1)
	grs.Go(func(ctx context.Context) {
		d.watch()
		d.wg.Done()
	})
}

// watch 持续监听服务列表变化
func (d *discovery) watch() {
	for !d.Stop() {
		select {
		case <-d.ctx.Done():
			return
		default:
			if err := d.fetch(); err != nil {
				select {
				case <-d.ctx.Done():
					return
				case <-time.After(time.Second):
				}
			}
		}
	}
}

// fetch 获取服务列表并更新 watchers
func (d *discovery) fetch() error {
	options := &api.QueryOptions{
		WaitIndex: d.waitIndex,
		WaitTime:  d.config.WatchWaitTime,
	}
	options = options.WithContext(d.ctx)

	services, meta, err := d.client.Catalog().Services(options)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			glog.Error("consul获取服务列表失败", zap.Error(err))
		}
		return err
	}

	d.waitIndex = meta.LastIndex

	// 创建新的服务 watcher  只增不减 确保listener不会被删除
	for name := range services {
		if name == "consul" {
			continue
		}
		_ = d.getOrCreateWatcher(name)
	}
	return nil
}

func (d *discovery) getWatcher(name string) *Watcher {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.watchers[name]
}

func (d *discovery) addWatcher(name string, watcher *Watcher) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.watchers[name] = watcher
}

func (d *discovery) delWatcher(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.watchers, name)
}

func (d *discovery) rangeWatcher(f func(watcher *Watcher) bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, watcher := range d.watchers {
		if !f(watcher) {
			return
		}
	}
}

func (d *discovery) getOrCreateWatcher(name string) *Watcher {
	var ok bool
	watcher := d.getWatcher(name)
	if watcher != nil {
		return watcher
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if watcher, ok = d.watchers[name]; ok {
		return watcher
	}

	watcher = newWatcher(d.ctx, d.wg, d.client, d.config, name)
	d.watchers[name] = watcher
	return watcher
}

func (d *discovery) Watch(kind string, listener iface.ServiceChangeHandler) {
	watcher := d.getOrCreateWatcher(kind)
	watcher.listener.Register(listener)
}

func (d *discovery) Unwatch(kind string, listener iface.ServiceChangeHandler) {
	watcher := d.getWatcher(kind)
	if watcher == nil {
		return
	}
	watcher.listener.UnRegister(listener)
}

func (d *discovery) GetByKind(kind string) map[uint64]*iface.Member {
	watcher := d.getWatcher(kind)
	if watcher == nil {
		return nil
	}
	return watcher.GetAll()
}

func (d *discovery) GetAll() map[uint64]*iface.Member {
	var result = make(map[uint64]*iface.Member)
	d.rangeWatcher(func(watcher *Watcher) bool {
		result = maputil.Merge(result, watcher.GetAll())
		return true
	})
	return result
}

func (d *discovery) GetById(memberId uint64) *iface.Member {
	var result *iface.Member
	d.rangeWatcher(func(watcher *Watcher) bool {
		result = watcher.GetById(memberId)
		return result == nil
	})
	return result
}

func (d *discovery) Shutdown() error {
	if !d.Stop() {
		return nil
	}
	d.cancel()
	return nil
}
