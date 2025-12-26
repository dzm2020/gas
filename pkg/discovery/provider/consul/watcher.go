package consul

import (
	"context"
	"gas/pkg/discovery/iface"
	"gas/pkg/glog"
	"gas/pkg/lib/grs"
	"sync/atomic"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

func newWatcher(ctx context.Context, client *api.Client, kind string, config *Config) *Watcher {
	watcher := &Watcher{
		ctx:                    ctx,
		client:                 client,
		config:                 config,
		waitIndex:              0,
		serviceListenerManager: newServiceListenerManager(),
		kind:                   kind,
	}
	watcher.list.Store(iface.NewMemberList(nil))

	grs.Go(func(ctx context.Context) {
		watcher.loop()
	})

	return watcher
}

type Watcher struct {
	ctx context.Context
	*serviceListenerManager
	client    *api.Client
	config    *Config
	waitIndex uint64
	list      atomic.Pointer[iface.MemberList] // 并发读写
	kind      string
}

func (w *Watcher) loop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		default:
			if err := w.fetch(w.Notify); err != nil {
				select {
				case <-time.After(time.Second):
				case <-w.ctx.Done():
					return
				}
			}
		}
	}
}

func (w *Watcher) fetch(listener func(*iface.Topology)) error {
	kind := w.kind
	options := &api.QueryOptions{
		WaitIndex: w.waitIndex,
		WaitTime:  w.config.WatchWaitTime,
	}
	options = options.WithContext(w.ctx)
	services, meta, err := w.client.Health().Service(kind, "", true, options)
	if err != nil {
		glog.Error("Consul获取服务失败", zap.String("service", kind), zap.Error(err))
		return err
	}

	w.waitIndex = meta.LastIndex

	nodeDict := make(map[uint64]*iface.Member, len(services))
	for _, s := range services {
		id, err := convertor.ToInt(s.Service.ID)
		if err != nil {
			glog.Warn("Consul服务ID转换失败，跳过该服务", zap.String("serviceId", s.Service.ID), zap.String("kind", s.Service.Service), zap.Error(err))
			continue
		}
		if id <= 0 {
			glog.Warn("Consul服务ID无效，跳过该服务", zap.String("serviceId", s.Service.ID), zap.String("kind", s.Service.Service))
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
		if listener != nil {
			listener(topology)
		}
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
