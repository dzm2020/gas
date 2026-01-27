package consul

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dzm2020/gas/pkg/discovery/iface"
	"github.com/dzm2020/gas/pkg/glog"
	"github.com/dzm2020/gas/pkg/lib/event"
	"github.com/dzm2020/gas/pkg/lib/grs"
	"github.com/dzm2020/gas/pkg/lib/stopper"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

func newWatcher(ctx context.Context, wg *sync.WaitGroup, client *api.Client, config *Config, kind string) *Watcher {
	watcher := &Watcher{
		client:    client,
		config:    config,
		wg:        wg,
		waitIndex: 0,
		listener:  event.NewListener[*iface.Topology](),
		kind:      kind,
	}
	watcher.list.Store(iface.NewMemberList(nil))
	watcher.ctx, watcher.cancel = context.WithCancel(ctx)

	watcher.wg.Add(1)
	grs.Go(func(ctx context.Context) {
		watcher.loop()
		watcher.wg.Done()
	})
	return watcher
}

type Watcher struct {
	stopper.Stopper

	client *api.Client
	config *Config

	listener  *event.Listener[*iface.Topology]
	waitIndex uint64
	list      atomic.Pointer[iface.MemberList] // 并发读写
	kind      string

	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup
}

func (w *Watcher) loop() {
	defer func() {
		w.shutdown()
	}()
	for !w.IsStop() {
		select {
		case <-w.ctx.Done():
			return
		default:
			if err := w.fetch(); err != nil {
				select {
				case <-time.After(time.Second):
				case <-w.ctx.Done():
					return
				}
			}
		}
	}
}

func (w *Watcher) fetch() error {
	options := &api.QueryOptions{
		WaitIndex: w.waitIndex,
		WaitTime:  w.config.WatchWaitTime,
	}
	options = options.WithContext(w.ctx)
	services, meta, err := w.client.Health().Service(w.kind, "", true, options)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			glog.Error("Consul获取服务失败", zap.String("service", w.kind), zap.Error(err))
		}
		return err
	}

	w.waitIndex = meta.LastIndex

	nodeDict := make(map[uint64]*iface.Member, len(services))
	for _, s := range services {
		id, wrong := convertor.ToInt(s.Service.ID)
		if wrong != nil {
			glog.Warn("Consul服务ID无效，跳过该服务", zap.String("service", w.kind), zap.Error(wrong))
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

	glog.Debug("consul watch fetch", zap.Any("nodeDict", nodeDict))

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

func (w *Watcher) shutdown() {
	if !w.Stop() {
		return
	}
	w.cancel()
}
