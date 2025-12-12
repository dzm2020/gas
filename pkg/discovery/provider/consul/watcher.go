package consul

import (
	"context"
	"gas/pkg/discovery/iface"
	"gas/pkg/lib"
	"gas/pkg/lib/glog"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

func newConsulWatcher(client *api.Client, kind string, config *Config, stopCh <-chan struct{}) *consulWatcher {
	return &consulWatcher{
		client:                 client,
		config:                 config,
		stopCh:                 stopCh,
		waitIndex:              0,
		list:                   iface.NewMemberList(nil),
		serviceListenerManager: newServiceListenerManager(),
		kind:                   kind,
	}
}

type consulWatcher struct {
	*serviceListenerManager
	client    *api.Client
	config    *Config
	stopCh    <-chan struct{}
	waitIndex uint64
	list      *iface.MemberList
	kind      string
}

func (w *consulWatcher) start() {
	lib.Go(func(ctx context.Context) {
		w.loop(ctx, w.kind)
	})
}

func (w *consulWatcher) loop(ctx context.Context, service string) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
			if err := w.fetch(ctx, service, w.Notify); err != nil {
				select {
				case <-time.After(time.Second):
				case <-w.stopCh:
					return
				}
			}
		}
	}
}

func (w *consulWatcher) fetch(ctx context.Context, kind string, listener func(*iface.Topology)) error {
	options := &api.QueryOptions{
		WaitIndex: w.waitIndex,
		WaitTime:  w.config.WatchWaitTime,
	}
	options.WithContext(ctx)
	services, meta, err := w.client.Health().Service(kind, "", true, options)
	if err != nil {
		glog.Error("Consul获取服务失败", zap.String("service", kind), zap.Error(err))
		return err
	}

	w.waitIndex = meta.LastIndex

	nodeDict := make(map[uint64]*iface.Member, len(services))
	for _, s := range services {
		id, _ := convertor.ToInt(s.Service.ID)
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
	topology := list.UpdateTopology(w.list)
	if len(topology.Left) > 0 || len(topology.Joined) > 0 {
		glog.Debug("Consul服务拓扑变化",
			zap.String("kind", kind),
			zap.Int("joined", len(topology.Joined)),
			zap.Int("alive", len(topology.Alive)),
			zap.Int("left", len(topology.Left)),
			zap.Int("total", len(nodeDict)))

	} else {
		glog.Debug("Consul服务拓扑未变化",
			zap.String("service", kind),
			zap.Int("total", len(nodeDict)))
	}
	if listener != nil {
		listener(topology)
	}
	w.list = list
	return nil
}
