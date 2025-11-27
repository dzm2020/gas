package consul

import (
	"gas/pkg/discovery/iface"
	"gas/pkg/lib/glog"
	"sync"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/hashicorp/consul/api"
	"go.uber.org/zap"
)

type consulRegistrar struct {
	client   *api.Client
	opts     *Options
	stopCh   <-chan struct{}
	status   string
	wg       sync.WaitGroup
	healthMu sync.RWMutex
	healthCh map[uint64]chan string
}

func newConsulRegistrar(client *api.Client, opts *Options, stopCh <-chan struct{}) *consulRegistrar {
	return &consulRegistrar{
		client:   client,
		opts:     opts,
		stopCh:   stopCh,
		healthCh: make(map[uint64]chan string),
	}
}

func (r *consulRegistrar) Add(node *iface.Node) error {
	check := &api.AgentServiceCheck{
		TTL:                            r.opts.HealthTTL.String(),
		DeregisterCriticalServiceAfter: r.opts.DeregisterInterval.String(),
	}
	registration := &api.AgentServiceRegistration{
		ID:      convertor.ToString(node.GetID()),
		Name:    node.GetName(),
		Address: node.GetAddress(),
		Port:    node.GetPort(),
		Tags:    node.GetTags(),
		Meta:    node.GetMeta(),
		Check:   check,
	}
	if err := r.client.Agent().ServiceRegister(registration); err != nil {
		glog.Error("consul registrar: failed to register node",
			zap.Uint64("nodeId", node.GetID()),
			zap.String("name", node.GetName()),
			zap.Error(err))
		return err
	}

	ch := make(chan string, 1)
	if !r.setHealthChan(node.GetID(), ch) {
		glog.Debug("consul registrar: node already registered", zap.Uint64("nodeId", node.GetID()))
		return nil // service already registered
	}

	r.wg.Add(1)
	go r.healthCheck(node.GetID(), ch)
	glog.Info("consul registrar: node registered successfully",
		zap.Uint64("nodeId", node.GetID()),
		zap.String("name", node.GetName()))
	return nil
}

func (r *consulRegistrar) UpdateStatus(nodeID uint64, status string) {
	ch := r.getHealthChan(nodeID)
	if ch == nil {
		return
	}
	select {
	case ch <- status:
	default:
		glog.Warn("consul registrar: failed to update node status, channel full", zap.Uint64("nodeId", nodeID))
	}
}

func (r *consulRegistrar) Remove(nodeID uint64) error {
	if r.getHealthChan(nodeID) == nil {
		return nil
	}
	r.deleteHealthChan(nodeID)
	if err := r.client.Agent().ServiceDeregister(convertor.ToString(nodeID)); err != nil {
		glog.Error("consul registrar: failed to remove node", zap.Uint64("nodeId", nodeID), zap.Error(err))
		return err
	}
	glog.Info("consul registrar: node removed successfully", zap.Uint64("nodeId", nodeID))
	return nil
}

func (r *consulRegistrar) healthCheck(nodeID uint64, ch <-chan string) {
	defer func() {
		if rec := recover(); rec != nil {
			glog.Error("consul registrar healthCheck panic", zap.Uint64("nodeID", nodeID), zap.Any("error", rec))
		}
		r.wg.Done()
	}()
	interval := r.opts.HealthTTL / 2
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer func() {
		_ = r.Remove(nodeID)
	}()

	r.status = "pass"

	_ = r.updateTTL(nodeID, r.status)

	for {
		select {
		case <-r.stopCh:
			return
		case status, ok := <-ch:
			if !ok {
				return
			}
			r.status = status
			if err := r.updateTTL(nodeID, status); err != nil {
				glog.Error("consul registrar: failed to update TTL", zap.Uint64("nodeId", nodeID), zap.Error(err))
				return
			}
		case <-ticker.C:
			if err := r.updateTTL(nodeID, r.status); err != nil {
				glog.Error("consul registrar: failed to update TTL on tick", zap.Uint64("nodeId", nodeID), zap.Error(err))
				return
			}
		}
	}
}

func (r *consulRegistrar) updateTTL(nodeID uint64, status string) error {
	return r.client.Agent().UpdateTTL("service:"+convertor.ToString(nodeID), "", status)
}

func (r *consulRegistrar) setHealthChan(nodeID uint64, ch chan string) bool {
	r.healthMu.Lock()
	defer r.healthMu.Unlock()
	if _, ok := r.healthCh[nodeID]; ok {
		return false
	}
	r.healthCh[nodeID] = ch
	return true
}

func (r *consulRegistrar) getHealthChan(nodeID uint64) chan string {
	r.healthMu.RLock()
	defer r.healthMu.RUnlock()
	return r.healthCh[nodeID]
}

func (r *consulRegistrar) deleteHealthChan(nodeID uint64) {
	r.healthMu.Lock()
	defer r.healthMu.Unlock()
	if ch, ok := r.healthCh[nodeID]; ok {
		close(ch)
		delete(r.healthCh, nodeID)
	}
}

func (r *consulRegistrar) shutdown() {
	var nodes []uint64
	r.healthMu.Lock()
	for id, _ := range r.healthCh {
		nodes = append(nodes, id)
	}
	r.healthMu.Unlock()
	for _, id := range nodes {
		_ = r.Remove(id)
	}
}

func (r *consulRegistrar) Wait() {
	r.wg.Wait()
}
