package consul

import (
	"gas/pkg/discovery/iface"
	"gas/pkg/utils/glog"
	"sync"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

func newConsulWatcher(client *api.Client, service string, opts *Options, stopCh <-chan struct{}) *consulWatcher {
	return &consulWatcher{
		client:                 client,
		opts:                   opts,
		stopCh:                 stopCh,
		waitIndex:              0,
		list:                   iface.NewNodeList(nil),
		serviceListenerManager: newServiceListenerManager(),
		service:                service,
	}
}

type consulWatcher struct {
	*serviceListenerManager
	client    *api.Client
	opts      *Options
	stopCh    <-chan struct{}
	waitIndex uint64
	wg        sync.WaitGroup
	list      *iface.NodeList
	service   string
}

func (w *consulWatcher) start() {
	w.wg.Add(1)
	go w.loop(w.service)
}

func (w *consulWatcher) loop(service string) {
	defer func() {
		if rec := recover(); rec != nil {
			glog.Error("consul watcher loop panic", zap.String("service", service), zap.Any("error", rec))
		}
		w.wg.Done()
	}()
	for {
		select {
		case <-w.stopCh:
			return
		default:
			if err := w.fetch(service, w.serviceListenerManager.Notify); err != nil {
				select {
				case <-time.After(time.Second):
				case <-w.stopCh:
					return
				}
			}
		}
	}
}

func (w *consulWatcher) fetch(service string, listener func(*iface.Topology)) error {
	services, meta, err := w.client.Health().Service(service, "", true, &api.QueryOptions{
		WaitIndex: w.waitIndex,
		WaitTime:  w.opts.WatchWaitTime,
	})
	if err != nil {
		glog.Error("consul watcher: failed to fetch service", zap.String("service", service), zap.Error(err))
		return err
	}

	w.waitIndex = meta.LastIndex

	nodeDict := make(map[uint64]*iface.Node, len(services))
	for _, s := range services {
		id, _ := convertor.ToInt(s.Service.ID)
		nodeDict[uint64(id)] = &iface.Node{
			Id:      uint64(id),
			Name:    s.Service.Service,
			Address: s.Service.Address,
			Port:    s.Service.Port,
			Tags:    s.Service.Tags,
			Meta:    s.Service.Meta,
		}
	}

	list := iface.NewNodeList(nodeDict)
	topology := list.UpdateTopology(w.list)
	
	if len(topology.Left) > 0 || len(topology.Joined) > 0 {
		glog.Info("consul watcher: service topology changed",
			zap.String("service", service),
			zap.Int("joined", len(topology.Joined)),
			zap.Int("alive", len(topology.Alive)),
			zap.Int("left", len(topology.Left)),
			zap.Int("total", len(nodeDict)))
		if listener != nil {
			listener(topology)
		}
	} else {
		glog.Debug("consul watcher: service topology unchanged",
			zap.String("service", service),
			zap.Int("total", len(nodeDict)))
	}
	
	w.list = list
	return nil
}

func (w *consulWatcher) Wait() {
	w.wg.Wait()
}
