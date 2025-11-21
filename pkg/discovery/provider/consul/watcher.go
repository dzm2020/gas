package consul

import (
	"gas/pkg/discovery/iface"
	"gas/pkg/utils/glog"
	"reflect"
	"sync"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

func newConsulWatcher(client *api.Client, service string, opts *Options, stopCh <-chan struct{}) *consulWatcher {
	return &consulWatcher{
		client:    client,
		opts:      opts,
		stopCh:    stopCh,
		waitIndex: 0,
		list:      iface.NewNodeList(nil),
		handlers:  make([]iface.UpdateHandler, 0),
		service:   service,
	}
}

type consulWatcher struct {
	client     *api.Client
	opts       *Options
	stopCh     <-chan struct{}
	waitIndex  uint64
	wg         sync.WaitGroup
	list       *iface.NodeList // 只要写完后才会被读到所以不存在并发读写问题
	handlersMu sync.RWMutex
	handlers   []iface.UpdateHandler
	service    string
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
			if err := w.fetch(service, w.callAllHandlers); err != nil {
				select {
				case <-time.After(time.Second):
				case <-w.stopCh:
					return
				}
			}
		}
	}
}

func (w *consulWatcher) callAllHandlers(topology *iface.Topology) {
	w.handlersMu.RLock()
	handlers := make([]iface.UpdateHandler, len(w.handlers))
	copy(handlers, w.handlers)
	w.handlersMu.RUnlock()

	// 调用所有注册的 handlers
	for _, handler := range handlers {
		if handler != nil {
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						glog.Error("consul watcher handler panic", zap.Any("error", rec))
					}
				}()
				handler(topology)
			}()
		}
	}
}

func (w *consulWatcher) fetch(service string, handler func(*iface.Topology)) error {
	opt := &api.QueryOptions{
		WaitIndex: w.currentIndex(),
		WaitTime:  w.opts.WatchWaitTime,
	}

	services, meta, err := w.client.Health().Service(service, "", true, opt)
	if err != nil {
		return err
	}
	nodeDict := make(map[uint64]*iface.Node)
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
	w.setIndex(meta.LastIndex)
	list := iface.NewNodeList(nodeDict)

	topology := list.UpdateTopology(w.list)
	if len(topology.Left) != 0 || len(topology.Joined) != 0 {
		if handler != nil {
			handler(topology)
		}
	}
	w.list = list

	return nil
}

func (w *consulWatcher) currentIndex() uint64 {
	return w.waitIndex
}

func (w *consulWatcher) setIndex(idx uint64) {
	w.waitIndex = idx
}

func (w *consulWatcher) Wait() {
	w.wg.Wait()
}

func (w *consulWatcher) WatchNode(service string, handler iface.UpdateHandler) error {
	if handler == nil {
		return nil
	}
	w.handlersMu.Lock()
	w.service = service
	w.handlers = append(w.handlers, handler)
	w.handlersMu.Unlock()
	return nil
}

// RemoveHandler 移除指定的 handler
// 通过函数指针比较来识别要删除的 handler
func (w *consulWatcher) RemoveHandler(handler iface.UpdateHandler) bool {
	if handler == nil {
		return false
	}

	w.handlersMu.Lock()
	defer w.handlersMu.Unlock()

	handlerPtr := reflect.ValueOf(handler).Pointer()
	newHandlers := make([]iface.UpdateHandler, 0, len(w.handlers))
	removed := false

	for _, h := range w.handlers {
		if h == nil {
			continue
		}
		hPtr := reflect.ValueOf(h).Pointer()
		if hPtr == handlerPtr {
			removed = true
			continue
		}
		newHandlers = append(newHandlers, h)
	}

	if removed {
		w.handlers = newHandlers
	}

	return removed
}

// HandlerCount 返回当前注册的 handler 数量
func (w *consulWatcher) HandlerCount() int {
	w.handlersMu.RLock()
	defer w.handlersMu.RUnlock()
	return len(w.handlers)
}

func (w *consulWatcher) GetById(nodeId uint64) *iface.Node {
	v, _ := w.list.Dict[nodeId]
	return v
}

func (w *consulWatcher) GetByKind(kind string) (result []*iface.Node) {
	for _, node := range w.list.Dict {
		if slices.Contains(node.GetTags(), kind) {
			result = append(result, convertor.DeepClone(node))
		}
	}
	return
}

func (w *consulWatcher) GetAll() (result []*iface.Node) {
	for _, node := range w.list.Dict {
		result = append(result, convertor.DeepClone(node))
	}
	return
}
