package consul

import (
	"context"
	"errors"
	"github.com/dzm2020/gas/pkg/discovery/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/event"
	"github.com/dzm2020/gas/pkg/lib/grs"
	"sync/atomic"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

func newWatcher(provider *Provider, kind string) *Watcher {
	watcher := &Watcher{
		provider:  provider,
		waitIndex: 0,
		listener:  event.NewListener[*iface.Topology](),
		kind:      kind,
	}
	watcher.list.Store(iface.NewMemberList(nil))
	watcher.ctx, watcher.cancel = context.WithCancel(provider.ctx)
	grs.Go(func(ctx context.Context) {
		glog.Debug("服务监控协程运行", zap.String("service", kind))
		watcher.loop()
		glog.Debug("服务监控协程退出", zap.String("service", kind))
	})
	return watcher
}

type Watcher struct {
	provider  *Provider
	listener  *event.Listener[*iface.Topology]
	waitIndex uint64
	list      atomic.Pointer[iface.MemberList] // 并发读写
	kind      string
	ctx       context.Context
	cancel    context.CancelFunc
}

func (w *Watcher) loop() {
	ctx := w.ctx
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := w.fetch(); err != nil {
				select {
				case <-time.After(time.Second):
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (w *Watcher) fetch() error {
	config := w.provider.config
	ctx := w.ctx

	kind := w.kind
	options := &api.QueryOptions{
		WaitIndex: w.waitIndex,
		WaitTime:  config.WatchWaitTime,
	}
	options = options.WithContext(ctx)
	services, meta, err := w.provider.Health().Service(kind, "", true, options)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			glog.Error("Consul获取服务失败", zap.String("service", kind), zap.Error(err))
		}
		return err
	}

	w.waitIndex = meta.LastIndex

	nodeDict := make(map[uint64]*iface.Member, len(services))
	for _, s := range services {
		id, wrong := convertor.ToInt(s.Service.ID)
		if wrong != nil {
			glog.Warn("Consul服务ID无效，跳过该服务", zap.String("service", kind), zap.Error(wrong))
			continue
		}
		nodeDict[uint64(id)] = &iface.Member{
			Id:      uint64(id),
			Kind:    s.Service.Service,
			Address: s.Service.Address,
			Port:    s.Service.Port,
			Tags:    s.Service.Tags,
			Meta:    s.Service.Meta,
		}
	}

	list := iface.NewMemberList(nodeDict)

	old := w.list.Load()
	topology := list.UpdateTopology(old)

	w.list.Store(list)

	if topology.IsChange() {
		w.listener.Notify(topology)

	}
	return nil
}

func (w *Watcher) GetAll() map[uint64]*iface.Member {
	old := w.list.Load()
	if old == nil {
		return nil
	}
	return old.Dict
}

func (w *Watcher) GetById(id uint64) *iface.Member {
	old := w.list.Load()
	if old == nil {
		return nil
	}
	return old.Dict[id]
}

func (w *Watcher) Stop() {
	w.cancel()
}
